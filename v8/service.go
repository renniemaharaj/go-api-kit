package v8

import (
	"context"
	"fmt"
	"time"
)

// Repository is the read-only storage contract consumed by the health service.
type Repository interface {
	Read(context.Context) (Snapshot, error)
}

type service struct {
	repository Repository
}

var _ Service = (*service)(nil)

func (s *service) Check(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, fmt.Errorf("health: check: %w", err)
	}

	snapshot, err := s.repository.Read(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("health: check: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return Status{}, fmt.Errorf("health: check: %w", err)
	}

	return Status{
		Status:        "ok",
		UptimeSeconds: max(0, int64(time.Since(snapshot.StartedAt)/time.Second)),
	}, nil
}
