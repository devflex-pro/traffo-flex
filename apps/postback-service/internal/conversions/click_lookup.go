package conversions

import (
	"context"
	"log/slog"
)

type ClickInfo struct {
	CampaignID    string
	StreamID      string
	DestinationID string
	SourceID      string
}

type ClickLookup interface {
	Find(
		ctx context.Context,
		clickID string,
	) (
		ClickInfo,
		bool,
		error,
	)
}

type NoopClickLookup struct{}

func (NoopClickLookup) Find(
	context.Context,
	string,
) (
	ClickInfo,
	bool,
	error,
) {
	return ClickInfo{}, false, nil
}

type BestEffortClickLookup struct {
	log  *slog.Logger
	next ClickLookup
}

func NewBestEffortClickLookup(
	log *slog.Logger,
	next ClickLookup,
) *BestEffortClickLookup {
	return &BestEffortClickLookup{
		log:  log,
		next: next,
	}
}

func (l *BestEffortClickLookup) Find(
	ctx context.Context,
	clickID string,
) (
	ClickInfo,
	bool,
	error,
) {
	info, found, err := l.next.Find(
		ctx,
		clickID,
	)
	if err != nil {
		l.log.Warn(
			"click lookup failed",
			slog.String(
				"click_id",
				clickID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)
		return ClickInfo{}, false, nil
	}
	return info, found, nil
}
