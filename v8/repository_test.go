package v8

import (
	"context"
	"errors"
	"testing"
)

func TestRepositoryKeepsStartupSnapshotAndHonorsContext(t *testing.T) {
	storage := newRepository()
	first, err := storage.Read(context.Background())
	if err != nil || first.StartedAt.IsZero() {
		t.Fatalf("missing startup snapshot: %#v, %v", first, err)
	}

	second, err := storage.Read(context.Background())
	if err != nil || second != first {
		t.Fatalf("startup snapshot changed: %#v, %v", second, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := storage.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled repository read returned %v", err)
	}
}
