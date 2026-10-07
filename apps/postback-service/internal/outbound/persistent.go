package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"github.com/devflex/traffoflex/packages/go-shared/postback"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type PersistentWorker struct {
	db            *mongo.Database
	jobs          *mongo.Collection
	clickhouseURL string
	client        *http.Client
	lookupClient  *http.Client
	log           *slog.Logger
}

type Delivery struct {
	ID         string `bson:"_id"`
	TemplateID string `bson:"template_id"`
	OwnerID    string `bson:"owner_id"`
	URL        string `bson:"url"`
	Attempts   int    `bson:"attempts"`
	LeaseToken string `bson:"lease_token"`
}

func NewPersistentWorker(
	ctx context.Context,
	db *mongo.Database,
	clickhouseURL string,
	log *slog.Logger,
) (
	*PersistentWorker,
	error,
) {
	jobs := db.Collection(
		"outbound_postbacks",
		options.Collection().SetWriteConcern(writeconcern.Journaled()),
	)
	for collection, index := range map[*mongo.Collection]mongo.IndexModel{
		jobs:                         {Keys: bson.D{{Key: "status", Value: 1}, {Key: "next_attempt_at", Value: 1}}, Options: options.Index().SetName("outbound_pending_delivery")},
		db.Collection("conversions"): {Keys: bson.D{{Key: "outbound_status", Value: 1}, {Key: "outbound_next_attempt_at", Value: 1}}, Options: options.Index().SetName("conversions_pending_outbound")},
	} {
		if _, err := collection.Indexes().CreateOne(
			ctx,
			index,
		); err != nil {
			return nil, err
		}
	}
	if _, err := jobs.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("outbound_owner_created")},
	); err != nil {
		return nil, err
	}
	return &PersistentWorker{db: db, jobs: jobs, clickhouseURL: clickhouseURL, client: NewPublicClient(), lookupClient: &http.Client{Timeout: 5 * time.Second}, log: log}, nil
}

func (w *PersistentWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := w.plan(ctx); err != nil && ctx.Err() == nil {
			w.log.Warn(
				"outbound planning failed",
				"error",
				err,
			)
		}
		if _, err := w.Retry(ctx); err != nil && ctx.Err() == nil {
			w.log.Warn(
				"outbound delivery failed",
				"error",
				err,
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func decode[T any](doc bson.M) (
	T,
	error,
) {
	var value T
	payload, err := json.Marshal(doc)
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(
		payload,
		&value,
	)
	return value, err
}

func (w *PersistentWorker) plan(ctx context.Context) error {
	cursor, err := w.db.Collection("conversions").Find(
		ctx,
		bson.M{"outbound_status": "pending", "outbound_next_attempt_at": bson.M{"$lte": time.Now().UTC()}},
		options.Find().SetSort(bson.D{{Key: "outbound_next_attempt_at", Value: 1}}).SetLimit(100),
	)
	if err != nil {
		return err
	}
	var events []bson.M
	if err := cursor.All(
		ctx,
		&events,
	); err != nil {
		return err
	}
	for _, doc := range events {
		event, err := decode[models.ConversionEvent](doc)
		if err != nil {
			return err
		}
		if err := w.planEvent(
			ctx,
			event,
		); err != nil {
			return err
		}
	}
	return nil
}

func (w *PersistentWorker) planEvent(
	ctx context.Context,
	event models.ConversionEvent,
) error {
	cursor, err := w.db.Collection("postback_templates").Find(
		ctx,
		bson.M{"owner_id": event.OwnerID, "direction": "outgoing", "enabled": true},
	)
	if err != nil {
		return err
	}
	var docs []bson.M
	if err := cursor.All(
		ctx,
		&docs,
	); err != nil {
		return err
	}
	templates := []models.PostbackTemplate{}
	for _, doc := range docs {
		template, err := decode[models.PostbackTemplate](doc)
		if err != nil {
			return err
		}
		if !template.CreatedAt.After(event.CreatedAt) {
			templates = append(
				templates,
				template,
			)
		}
	}
	status := "skipped"
	if len(templates) > 0 && event.OwnerID != "" && Approved(event.Status) {
		click, found, err := w.lookup(
			ctx,
			event,
		)
		if err != nil || !found {
			_, updateErr := w.db.Collection("conversions").UpdateOne(
				ctx,
				bson.M{"_id": event.ConversionID, "outbound_status": "pending"},
				bson.M{"$set": bson.M{"outbound_next_attempt_at": time.Now().UTC().Add(30 * time.Second)}},
			)
			if updateErr != nil {
				return updateErr
			}
			if err != nil {
				w.log.Warn(
					"outbound attribution delayed",
					"conversion_id",
					event.ConversionID,
					"error",
					err,
				)
			}
			return nil
		}
		for _, template := range templates {
			if !Matches(
				template,
				event,
				click,
			) {
				continue
			}
			values := Values(
				event,
				click,
			)
			target, renderErr := postback.Render(
				template.URL,
				values,
			)
			if postback.UsesSourceClickID(template.URL) && click.SourceClickID == "" {
				renderErr = errors.New("source click ID is missing; pass it to the campaign tracking URL")
			}
			jobID := event.ConversionID + ":" + template.ID
			now := time.Now().UTC()
			job := bson.M{"id": jobID, "owner_id": event.OwnerID, "template_id": template.ID, "conversion_id": event.ConversionID, "click_id": event.ClickID, "url": target, "status": "pending", "attempts": 0, "next_attempt_at": now, "created_at": now, "updated_at": now}
			if renderErr != nil {
				job["status"] = "failed"
				job["error"] = renderErr.Error()
			}
			if _, err := w.jobs.UpdateOne(
				ctx,
				bson.M{"_id": jobID},
				bson.M{"$setOnInsert": job},
				options.UpdateOne().SetUpsert(true),
			); err != nil && !mongo.IsDuplicateKeyError(err) {
				return err
			}
			status = "queued"
		}
	}
	_, err = w.db.Collection("conversions").UpdateOne(
		ctx,
		bson.M{"_id": event.ConversionID, "outbound_status": "pending"},
		bson.M{"$set": bson.M{"outbound_status": status}},
	)
	return err
}

func Matches(
	template models.PostbackTemplate,
	event models.ConversionEvent,
	click Click,
) bool {
	return template.Direction == "outgoing" && template.Enabled && template.OwnerID == event.OwnerID && click.OwnerID == event.OwnerID &&
		!template.CreatedAt.After(event.CreatedAt) && (template.SourceID == "" || template.SourceID == click.SourceID) &&
		(template.CampaignID == "" || template.CampaignID == click.CampaignID)
}

func Approved(status string) bool {
	switch strings.ToLower(status) {
	case "approved", "sale", "confirmed":
		return true
	default:
		return false
	}
}

func Values(
	event models.ConversionEvent,
	click Click,
) map[string]string {
	values := map[string]string{"click_id": event.ClickID, "cid": event.ClickID, "source_click_id": click.SourceClickID, "conversion_id": event.ConversionID, "transaction_id": event.TransactionID, "event_type": event.EventType, "status": event.Status, "currency": event.Currency, "network_id": event.NetworkID, "payout": postback.Payout(event.Payout), "sum": postback.Payout(event.Payout), "revenue": postback.Payout(event.Payout)}
	for i, sub := range []string{click.Sub1, click.Sub2, click.Sub3, click.Sub4, click.Sub5, click.Sub6, click.Sub7, click.Sub8, click.Sub9, click.Sub10} {
		values["sub"+strconv.Itoa(i+1)] = sub
		if i < 4 {
			values["s"+strconv.Itoa(i+1)] = sub
		}
	}
	return values
}

func RetryDelay(attempts int) time.Duration {
	return time.Duration(5*(1<<min(
		max(
			attempts-1,
			0,
		),
		6,
	))) * time.Second
}

func (w *PersistentWorker) Retry(ctx context.Context) (
	int,
	error,
) {
	delivered := 0
	for count := 0; count < 100; count++ {
		if err := ctx.Err(); err != nil {
			return delivered, err
		}
		now := time.Now().UTC()
		lease := ids.New("lease")
		var job Delivery
		err := w.jobs.FindOneAndUpdate(
			ctx,
			bson.M{"$or": bson.A{bson.M{"status": "pending", "next_attempt_at": bson.M{"$lte": now}}, bson.M{"status": "sending", "lease_until": bson.M{"$lte": now}}}},
			bson.M{"$set": bson.M{"status": "sending", "lease_token": lease, "lease_until": now.Add(time.Minute), "updated_at": now}, "$inc": bson.M{"attempts": 1}},
			options.FindOneAndUpdate().SetSort(bson.D{{Key: "next_attempt_at", Value: 1}}).SetReturnDocument(options.After),
		).Decode(&job)
		if errors.Is(
			err,
			mongo.ErrNoDocuments,
		) {
			return delivered, nil
		}
		if err != nil {
			return delivered, err
		}
		var config bson.M
		configErr := w.db.Collection("postback_templates").FindOne(
			ctx,
			bson.M{"id": job.TemplateID, "owner_id": job.OwnerID, "direction": "outgoing", "enabled": true},
		).Decode(&config)
		if configErr != nil && !errors.Is(
			configErr,
			mongo.ErrNoDocuments,
		) {
			return delivered, configErr
		}
		status, code, message := "delivered", 0, ""
		if errors.Is(
			configErr,
			mongo.ErrNoDocuments,
		) {
			status, message = "cancelled", "postback template was disabled or deleted"
		} else if job.Attempts > 5 {
			status, message = "failed", "postback attempt limit reached after interrupted delivery"
		} else {
			code, err = Send(
				ctx,
				w.client,
				job.URL,
				job.ID,
			)
			if err != nil {
				status, message = "pending", err.Error()
				if job.Attempts >= 5 {
					status = "failed"
				}
			}
		}
		if _, err := w.jobs.UpdateOne(
			ctx,
			bson.M{"_id": job.ID, "lease_token": job.LeaseToken, "status": "sending"},
			bson.M{"$set": bson.M{"status": status, "error": message, "http_status": code, "next_attempt_at": time.Now().UTC().Add(RetryDelay(job.Attempts)), "updated_at": time.Now().UTC()}, "$unset": bson.M{"lease_until": "", "lease_token": ""}},
		); err != nil {
			return delivered, err
		}
		if status == "delivered" {
			delivered++
		}
	}
	return delivered, nil
}
