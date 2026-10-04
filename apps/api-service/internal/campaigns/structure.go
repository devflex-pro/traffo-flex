package campaigns

import (
	"context"
	"net/http"
	"sort"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type StructureStreamReader interface {
	ListAll(ctx context.Context) (
		[]models.Stream,
		error,
	)
}

type CampaignStructure struct {
	CampaignID       string `json:"campaign_id"`
	StreamCount      int    `json:"stream_count"`
	DestinationCount int    `json:"destination_count"`
}

func (h *Handler) SetStructureReader(streams StructureStreamReader) {
	h.streams = streams
}

func (h *Handler) Structure(
	w http.ResponseWriter,
	r *http.Request,
) {
	var streams []models.Stream
	if h.streams != nil {
		var err error
		streams, err = h.streams.ListAll(r.Context())
		if err != nil {
			h.respondError(
				w,
				http.StatusInternalServerError,
				"failed to load campaign structure",
				err,
			)
			return
		}
	}
	items := buildCampaignStructure(streams)
	if err := httpx.JSON(
		w,
		http.StatusOK,
		map[string]any{"items": items},
	); err != nil {
		h.log.Error(
			"failed to write campaign structure",
			"error",
			err,
		)
	}
}

func buildCampaignStructure(streams []models.Stream) []CampaignStructure {
	byCampaign := make(map[string]*CampaignStructure)
	seenDestinations := make(map[string]map[string]struct{})
	for _, stream := range streams {
		item := byCampaign[stream.CampaignID]
		if item == nil {
			item = &CampaignStructure{CampaignID: stream.CampaignID}
			byCampaign[stream.CampaignID] = item
			seenDestinations[stream.CampaignID] = make(map[string]struct{})
		}
		item.StreamCount++
		for _, target := range stream.Distribution.Destinations {
			if target.DestinationID == "" {
				continue
			}
			if _, seen := seenDestinations[stream.CampaignID][target.DestinationID]; seen {
				continue
			}
			seenDestinations[stream.CampaignID][target.DestinationID] = struct{}{}
			item.DestinationCount++
		}
	}
	items := make(
		[]CampaignStructure,
		0,
		len(byCampaign),
	)
	for _, item := range byCampaign {
		items = append(items, *item)
	}
	sort.Slice(
		items,
		func(i, j int) bool {
			return items[i].CampaignID < items[j].CampaignID
		},
	)
	return items
}
