package clicklog

import (
	"context"
	"encoding/json"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type KafkaSink struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaSink(
	producer *eventstream.Producer,
	topic string,
) *KafkaSink {
	return &KafkaSink{producer: producer, topic: topic}
}

func (s *KafkaSink) Write(
	ctx context.Context,
	event models.ClickEvent,
) error {
	return s.WriteBatch(
		ctx,
		[]models.ClickEvent{event},
	)
}

func (s *KafkaSink) WriteBatch(
	ctx context.Context,
	events []models.ClickEvent,
) error {
	items := make([]eventstream.JSONEvent, 0, len(events))
	for _, event := range events {
		row, err := clickEventRow(event)
		if err != nil {
			return err
		}
		items = append(items, eventstream.JSONEvent{Key: event.ClickID, Value: row})
	}
	return s.producer.WriteJSONBatch(ctx, s.topic, items)
}

func clickEventRow(event models.ClickEvent) (clickRow, error) {
	query, err := json.Marshal(event.Query)
	if err != nil {
		return clickRow{}, err
	}
	return clickRow{
		CreatedAt:     eventstream.ClickHouseDateTime(event.CreatedAt),
		OwnerID:       event.OwnerID,
		ClickID:       event.ClickID,
		CampaignID:    event.CampaignID,
		StreamID:      event.StreamID,
		DestinationID: event.DestinationID,
		SourceID:      event.SourceID,
		SourceClickID: event.SourceClickID,
		IPHash:        event.IPHash,
		IPPrefix:      event.IPPrefix,
		UserAgent:     event.UserAgent,
		UAHash:        event.UAHash,
		GeoCountry:    event.GeoCountry,
		GeoRegion:     event.GeoRegion,
		City:          event.City,
		ASN:           event.ASN,
		ISP:           event.ISP,
		DeviceType:    event.DeviceType,
		OS:            event.OS,
		Browser:       event.Browser,
		Referrer:      event.Referrer,
		Sub1:          event.SubIDs[0],
		Sub2:          event.SubIDs[1],
		Sub3:          event.SubIDs[2],
		Sub4:          event.SubIDs[3],
		Sub5:          event.SubIDs[4],
		Sub6:          event.SubIDs[5],
		Sub7:          event.SubIDs[6],
		Sub8:          event.SubIDs[7],
		Sub9:          event.SubIDs[8],
		Sub10:         event.SubIDs[9],
		UTMSource:     event.UTM.Source,
		UTMMedium:     event.UTM.Medium,
		UTMCampaign:   event.UTM.Campaign,
		UTMContent:    event.UTM.Content,
		UTMTerm:       event.UTM.Term,
		Cost:          event.Cost,
		Currency:      event.Currency,
		IsBot:         boolToUInt8(event.IsBot),
		BotScore:      event.BotScore,
		RawQuery:      event.RawQuery,
		Query:         string(query),
	}, nil
}

type clickRow struct {
	CreatedAt     string  `json:"created_at"`
	OwnerID       string  `json:"owner_id"`
	ClickID       string  `json:"click_id"`
	CampaignID    string  `json:"campaign_id"`
	StreamID      string  `json:"stream_id"`
	DestinationID string  `json:"destination_id"`
	SourceID      string  `json:"source_id"`
	SourceClickID string  `json:"source_click_id"`
	IPHash        string  `json:"ip_hash"`
	IPPrefix      string  `json:"ip_prefix"`
	UserAgent     string  `json:"user_agent"`
	UAHash        string  `json:"ua_hash"`
	GeoCountry    string  `json:"geo_country"`
	GeoRegion     string  `json:"geo_region"`
	City          string  `json:"city"`
	ASN           string  `json:"asn"`
	ISP           string  `json:"isp"`
	DeviceType    string  `json:"device_type"`
	OS            string  `json:"os"`
	Browser       string  `json:"browser"`
	Referrer      string  `json:"referrer"`
	Sub1          string  `json:"sub1"`
	Sub2          string  `json:"sub2"`
	Sub3          string  `json:"sub3"`
	Sub4          string  `json:"sub4"`
	Sub5          string  `json:"sub5"`
	Sub6          string  `json:"sub6"`
	Sub7          string  `json:"sub7"`
	Sub8          string  `json:"sub8"`
	Sub9          string  `json:"sub9"`
	Sub10         string  `json:"sub10"`
	UTMSource     string  `json:"utm_source"`
	UTMMedium     string  `json:"utm_medium"`
	UTMCampaign   string  `json:"utm_campaign"`
	UTMContent    string  `json:"utm_content"`
	UTMTerm       string  `json:"utm_term"`
	Cost          float64 `json:"cost"`
	Currency      string  `json:"currency"`
	IsBot         uint8   `json:"is_bot"`
	BotScore      float64 `json:"bot_score"`
	RawQuery      string  `json:"raw_query"`
	Query         string  `json:"query"`
}

func boolToUInt8(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}
