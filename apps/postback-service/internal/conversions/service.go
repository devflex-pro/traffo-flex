package conversions

import (
	"context"
	"time"

	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

// Repository stores a conversion and its delivery state in one document.
type Repository interface {
	CreateOrGet(
		ctx context.Context,
		event models.ConversionEvent,
	) (models.ConversionEvent, bool, error)
	Pending(
		ctx context.Context,
		limit int,
	) ([]models.ConversionEvent, error)
	MarkDelivered(
		ctx context.Context,
		conversionID string,
	) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Process(
	ctx context.Context,
	conversion normalize.Conversion,
) (models.ConversionEvent, bool, error) {
	now := time.Now().UTC()
	event := models.ConversionEvent{
		OwnerID:       conversion.OwnerID,
		ConversionID:  ids.New("cnv"),
		ClickID:       conversion.ClickID,
		TransactionID: conversion.TransactionID,
		EventType:     conversion.EventType,
		Status:        conversion.Status,
		Payout:        conversion.Payout,
		Currency:      conversion.Currency,
		NetworkID:     conversion.NetworkID,
		RawPayload:    conversion.RawPayload,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.repo.CreateOrGet(
		ctx,
		event,
	)
}
