package redirects

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/antirepeat"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/distribution"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/eventqueue"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/requestctx"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/rules"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficback"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/macros"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log          *slog.Logger
	cache        *cache.Store
	rules        *rules.Engine
	selector     *distribution.Selector
	builder      *requestctx.Builder
	clicklog     clicklog.Logger
	tbSink       trafficevents.TrafficbackSink
	availability *availability.Evaluator
	antiRepeat   *antirepeat.Manager
}

func NewHandler(
	log *slog.Logger,
	store *cache.Store,
	builder *requestctx.Builder,
	logger clicklog.Logger,
	tbSink trafficevents.TrafficbackSink,
	evaluator *availability.Evaluator,
	antiRepeat *antirepeat.Manager,
) *Handler {
	if store == nil {
		store = cache.NewDemoStore()
	}
	if builder == nil {
		builder = requestctx.NewBuilder(nil)
	}
	if logger == nil {
		logger = clicklog.NewAsyncLogger(
			nil,
			100,
		)
	}
	return &Handler{
		log:          log,
		cache:        store,
		rules:        rules.NewEngine(),
		selector:     distribution.NewSelector(),
		builder:      builder,
		clicklog:     logger,
		tbSink:       tbSink,
		availability: evaluator,
		antiRepeat:   antiRepeat,
	}
}

func (h *Handler) Campaign(
	w http.ResponseWriter,
	r *http.Request,
) {
	slug := chi.URLParam(
		r,
		"campaignSlug",
	)
	campaign, err := h.cache.GetBySlug(slug)
	h.redirectCampaign(
		w,
		r,
		slug,
		campaign,
		err,
	)
}

func (h *Handler) Go(
	w http.ResponseWriter,
	r *http.Request,
) {
	publicID := chi.URLParam(
		r,
		"campaignPublicId",
	)
	campaign, err := h.cache.GetByPublicID(publicID)
	h.redirectCampaign(
		w,
		r,
		publicID,
		campaign,
		err,
	)
}

func (h *Handler) Token(
	w http.ResponseWriter,
	r *http.Request,
) {
	token := chi.URLParam(
		r,
		"publicToken",
	)
	campaign, err := h.cache.GetByToken(token)
	h.redirectCampaign(
		w,
		r,
		token,
		campaign,
		err,
	)
}

func (h *Handler) redirectCampaign(
	w http.ResponseWriter,
	r *http.Request,
	lookup string,
	campaign cache.CampaignConfig,
	err error,
) {
	if err != nil {
		if errors.Is(
			err,
			cache.ErrCampaignNotFound,
		) {
			http.Error(
				w,
				"campaign not found",
				http.StatusNotFound,
			)
			return
		}
		http.Error(
			w,
			"campaign lookup failed",
			http.StatusInternalServerError,
		)
		return
	}
	if campaign.Campaign.Status != models.StatusActive {
		http.Error(
			w,
			"campaign is not active",
			http.StatusNotFound,
		)
		return
	}

	clickID := ids.New("clk")
	baseCtx := h.builder.Build(
		r,
		requestctx.Input{
			ClickID:      clickID,
			CampaignID:   campaign.Campaign.ID,
			OwnerID:      campaign.Campaign.OwnerID,
			PricingModel: campaign.Campaign.PricingModel,
		},
	)
	stream, err := h.rules.MatchStream(
		campaign.Streams,
		rules.Context{Values: requestctx.Values(baseCtx)},
	)
	if err != nil {
		if errors.Is(
			err,
			rules.ErrNoMatchingStream,
		) {
			h.redirectTrafficback(
				w,
				r,
				campaign,
				clickID,
				models.TrafficbackNoMatchingStream,
			)
			return
		}
		http.Error(
			w,
			"stream matching failed",
			http.StatusInternalServerError,
		)
		return
	}

	candidates := h.availableDestinations(
		r.Context(),
		stream.Distribution,
		campaign.Destinations,
		requestctx.Values(baseCtx),
		clickID,
	)
	selectDestination := func(items []models.Destination) (models.Destination, error) {
		return h.selector.Select(
			h.effectiveDistribution(stream.Distribution),
			items,
			clickID,
		)
	}
	var destination models.Destination
	if stream.Distribution.UniquePolicy.Enabled && h.antiRepeat != nil {
		destination, err = h.antiRepeat.SelectAndMark(
			antirepeat.StreamKey{CampaignID: campaign.Campaign.ID, StreamID: stream.ID},
			stream.Distribution.UniquePolicy,
			requestctx.Values(baseCtx)[stream.Distribution.UniquePolicy.UserKey],
			candidates,
			time.Now().UTC(),
			selectDestination,
		)
	} else {
		destination, err = selectDestination(candidates)
	}
	if err != nil {
		if errors.Is(
			err,
			distribution.ErrNoDestination,
		) || errors.Is(err, antirepeat.ErrHistoryUnavailable) || errors.Is(err, antirepeat.ErrHistoryExhausted) {
			h.redirectTrafficback(
				w,
				r,
				campaign,
				clickID,
				models.TrafficbackNoDestination,
			)
			return
		}
		http.Error(
			w,
			"destination selection failed",
			http.StatusInternalServerError,
		)
		return
	}

	redirectCtx := h.builder.Build(
		r,
		requestctx.Input{
			ClickID:       clickID,
			OwnerID:       campaign.Campaign.OwnerID,
			CampaignID:    campaign.Campaign.ID,
			StreamID:      stream.ID,
			DestinationID: destination.ID,
			SourceID:      campaign.Campaign.TrafficSourceID,
			PricingModel:  campaign.Campaign.PricingModel,
		},
	)
	targetURL := macros.Render(destination.URL, map[string]string{
		"click_id": clickID,
	})
	targetURL = macros.Render(
		targetURL,
		requestctx.Values(redirectCtx),
	)

	if err := h.clicklog.Log(
		r.Context(),
		requestctx.ClickEvent(
			redirectCtx,
			time.Now().UTC(),
		),
	); err != nil && !errors.Is(err, eventqueue.ErrFull) && !errors.Is(err, eventqueue.ErrWALFull) && !errors.Is(err, eventqueue.ErrWALWrite) {
		h.log.Warn(
			"failed to enqueue click log",
			"error",
			err,
			"click_id",
			clickID,
		)
	}

	h.log.Info(
		"campaign redirect",
		"campaign_lookup",
		lookup,
		"click_id",
		clickID,
		"stream_id",
		stream.ID,
		"destination_id",
		destination.ID,
	)
	respondRedirect(
		h.log,
		w,
		r,
		targetURL,
		destination.Redirect.Mode,
	)
}

func (h *Handler) availableDestinations(
	ctx context.Context,
	distribution models.Distribution,
	destinations []models.Destination,
	values map[string]string,
	selectionKey string,
) []models.Destination {
	if h.availability == nil {
		return destinations
	}
	return h.availability.Filter(
		ctx,
		distribution,
		destinations,
		values,
		selectionKey,
	)
}

func (h *Handler) effectiveDistribution(distribution models.Distribution) models.Distribution {
	if !distribution.UniquePolicy.Enabled || distribution.UniquePolicy.SelectionStrategy == "" {
		return distribution
	}
	distribution.Mode = distribution.UniquePolicy.SelectionStrategy
	return distribution
}

func (h *Handler) Trafficback(
	w http.ResponseWriter,
	r *http.Request,
) {
	slug := chi.URLParam(
		r,
		"campaignSlug",
	)
	campaign, err := h.cache.GetBySlug(slug)
	if err != nil {
		if errors.Is(
			err,
			cache.ErrCampaignNotFound,
		) {
			http.Error(
				w,
				"campaign not found",
				http.StatusNotFound,
			)
			return
		}
		http.Error(
			w,
			"campaign lookup failed",
			http.StatusInternalServerError,
		)
		return
	}

	h.redirectTrafficback(
		w,
		r,
		campaign,
		firstNonEmpty(
			r.URL.Query().Get("click_id"),
			ids.New("clk"),
		),
		models.TrafficbackReason(firstNonEmpty(
			r.URL.Query().Get("reason"),
			string(models.TrafficbackNoMatchingStream),
		)),
	)
}

func (h *Handler) redirectTrafficback(
	w http.ResponseWriter,
	r *http.Request,
	campaign cache.CampaignConfig,
	clickID string,
	reason models.TrafficbackReason,
) {
	cfg := campaign.Campaign.TrafficbackConfig
	if !cfg.Enabled || cfg.URL == "" {
		http.Error(
			w,
			"trafficback is not configured",
			http.StatusServiceUnavailable,
		)
		return
	}

	guard := trafficback.NewGuard(cfg.MaxDepth)
	state := guard.Parse(r)
	if err := guard.Check(
		state,
		"",
	); err != nil {
		if errors.Is(
			err,
			trafficback.ErrLoopDetected,
		) {
			http.Error(
				w,
				"trafficback loop detected",
				http.StatusTooManyRequests,
			)
			return
		}
		http.Error(
			w,
			"trafficback guard failed",
			http.StatusInternalServerError,
		)
		return
	}
	nextState := guard.Next(
		state,
		"",
	)
	ctx := h.builder.Build(
		r,
		requestctx.Input{
			ClickID:      clickID,
			CampaignID:   campaign.Campaign.ID,
			OwnerID:      campaign.Campaign.OwnerID,
			PricingModel: campaign.Campaign.PricingModel,
		},
	)
	values := requestctx.Values(ctx)
	values["trafficback_reason"] = string(reason)
	values["trafficback_depth"] = intString(nextState.Depth)

	targetURL := macros.Render(
		cfg.URL,
		values,
	)
	h.logTrafficback(
		r,
		clickID,
		campaign.Campaign.OwnerID,
		campaign.Campaign.ID,
		reason,
		nextState,
	)
	h.log.Info(
		"trafficback redirect",
		"campaign_slug",
		campaign.Campaign.Slug,
		"click_id",
		clickID,
		"depth",
		nextState.Depth,
		"reason",
		values["trafficback_reason"],
	)
	http.Redirect(
		w,
		r,
		targetURL,
		http.StatusFound,
	)
}

func (h *Handler) logTrafficback(
	r *http.Request,
	clickID string,
	ownerID string,
	campaignID string,
	reason models.TrafficbackReason,
	state trafficback.State,
) {
	if h.tbSink == nil {
		return
	}
	if err := h.tbSink.WriteTrafficback(
		r.Context(),
		models.TrafficbackEvent{
			ClickID:             clickID,
			OwnerID:             ownerID,
			CampaignID:          campaignID,
			Reason:              reason,
			Depth:               state.Depth,
			VisitedDestinations: state.VisitedDestinations,
			CreatedAt:           time.Now().UTC(),
		},
	); err != nil && !errors.Is(err, eventqueue.ErrFull) {
		h.log.Warn(
			"failed to write trafficback event",
			"error",
			err,
			"click_id",
			clickID,
		)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func intString(value int) string {
	return strconv.Itoa(value)
}
