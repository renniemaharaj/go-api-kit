package v8

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryFunc func(context.Context) (Snapshot, error)

func (read repositoryFunc) Read(ctx context.Context) (Snapshot, error) {
	return read(ctx)
}

func TestServiceReadsRepositoryWithRequestContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	checks := service{repository: repositoryFunc(func(received context.Context) (Snapshot, error) {
		if received != ctx {
			t.Fatal("repository did not receive the request context")
		}

		return Snapshot{StartedAt: time.Now().Add(-10 * time.Second)}, nil
	})}

	status, err := checks.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if status.Status != "ok" || status.UptimeSeconds < 10 || status.UptimeSeconds > 11 {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestServicePreservesRepositoryErrors(t *testing.T) {
	failure := errors.New("storage unavailable")
	checks := service{repository: repositoryFunc(func(context.Context) (Snapshot, error) {
		return Snapshot{}, failure
	})}

	if _, err := checks.Check(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("repository error was lost: %v", err)
	}
}

func TestServiceHonorsCancellationBeforeAndAfterReading(t *testing.T) {
	for _, cancelBeforeRead := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		checks := service{repository: repositoryFunc(func(context.Context) (Snapshot, error) {
			calls++
			cancel()
			return Snapshot{StartedAt: time.Now()}, nil
		})}
		if cancelBeforeRead {
			cancel()
		}

		_, err := checks.Check(ctx)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was lost: %v", err)
		}

		if cancelBeforeRead && calls != 0 {
			t.Fatal("canceled request reached storage")
		}
		if !cancelBeforeRead && calls != 1 {
			t.Fatal("service did not read storage")
		}
	}
}
