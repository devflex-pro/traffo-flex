package conversions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrDuplicate = errors.New("duplicate conversion")

type Repository interface {
	Create(
		ctx context.Context,
		event models.ConversionEvent,
	) (
		models.ConversionEvent,
		error,
	)
	ExistsDedupe(
		ctx context.Context,
		key DedupeKey,
	) (
		bool,
		error,
	)
	MarkDedupe(
		ctx context.Context,
		key DedupeKey,
	) error
}

type DedupeKey struct {
	NetworkID     string
	TransactionID string
	EventType     string
	ClickID       string
}

type Service struct {
	repo        Repository
	sink        EventSink
	clickLookup ClickLookup
}

func NewService(repo Repository) *Service {
	return NewServiceWithSink(
		repo,
		NewMemoryEventSink(),
	)
}

func NewServiceWithSink(
	repo Repository,
	sink EventSink,
) *Service {
	return NewServiceWithDependencies(
		repo,
		sink,
		nil,
	)
}

func NewServiceWithDependencies(
	repo Repository,
	sink EventSink,
	clickLookup ClickLookup,
) *Service {
	if sink == nil {
		sink = NewMemoryEventSink()
	}
	if clickLookup == nil {
		clickLookup = NoopClickLookup{}
	}
	return &Service{
		repo:        repo,
		sink:        sink,
		clickLookup: clickLookup,
	}
}

func (s *Service) Process(
	ctx context.Context,
	conversion normalize.Conversion,
) (
	models.ConversionEvent,
	error,
) {
	key := DedupeKey{
		NetworkID:     conversion.NetworkID,
		TransactionID: conversion.TransactionID,
		EventType:     conversion.EventType,
		ClickID:       conversion.ClickID,
	}
	exists, err := s.repo.ExistsDedupe(
		ctx,
		key,
	)
	if err != nil {
		return models.ConversionEvent{}, err
	}
	if exists {
		return models.ConversionEvent{}, ErrDuplicate
	}
	if err := s.repo.MarkDedupe(
		ctx,
		key,
	); err != nil {
		if errors.Is(
			err,
			ErrDuplicate,
		) {
			return models.ConversionEvent{}, ErrDuplicate
		}
		return models.ConversionEvent{}, err
	}

	now := time.Now().UTC()
	event := models.ConversionEvent{
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
	clickInfo, found, err := s.clickLookup.Find(
		ctx,
		conversion.ClickID,
	)
	if err != nil {
		return models.ConversionEvent{}, err
	}
	if found {
		event.CampaignID = clickInfo.CampaignID
		event.StreamID = clickInfo.StreamID
		event.DestinationID = clickInfo.DestinationID
		event.SourceID = clickInfo.SourceID
	}
	if _, err := s.repo.Create(
		ctx,
		event,
	); err != nil {
		return models.ConversionEvent{}, err
	}
	if err := s.sink.Write(
		ctx,
		event,
	); err != nil {
		return models.ConversionEvent{}, err
	}
	return event, nil
}

func (k DedupeKey) String() string {
	return strings.Join(
		[]string{k.NetworkID, k.TransactionID, k.EventType, k.ClickID},
		"\x00",
	)
}
