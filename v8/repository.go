package v8

import (
	"context"
	"time"
)

// repository holds process-local state; it does not probe external dependencies.
type repository struct {
	snapshot Snapshot
}

var _ Repository = (*repository)(nil)

func newRepository() *repository {
	return &repository{
		snapshot: Snapshot{StartedAt: time.Now()},
	}
}

func (r *repository) Read(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	return r.snapshot, nil
}
