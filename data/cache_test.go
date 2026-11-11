package data_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
)

func TestCacheValidationAndOptIn(t *testing.T) {
	load := func(context.Context, string) (string, error) { return "Ada", nil }
	if _, err := data.NewCache[string, string](nil, data.CacheConfig[string]{}); err == nil {
		t.Fatal("missing loader was accepted")
	}

	for _, config := range []data.CacheConfig[string]{
		{TTL: -time.Second},
		{MaxEntries: -1},
	} {
		if _, err := data.NewCache(load, config); err == nil {
			t.Fatal("negative cache limits were accepted")
		}
	}

	for _, ttl := range []time.Duration{0, time.Minute} {
		loads := 0
		cached, err := data.NewCache(func(context.Context, string) (string, error) {
			loads++
			return "Ada", nil
		}, data.CacheConfig[string]{TTL: ttl})
		if err != nil {
			t.Fatal(err)
		}

		for range 2 {
			value, err := cached.Get(context.Background(), "42")
			if err != nil || value != "Ada" {
				t.Fatalf("unexpected load: %q, %v", value, err)
			}
		}

		want := 2
		if ttl > 0 {
			want = 1
		}

		if loads != want {
			t.Fatalf("TTL %s: %d loads, want %d", ttl, loads, want)
		}
	}
}

func TestCacheErrorsCancellationAndContext(t *testing.T) {
	type contextKey struct{}
	failure := errors.New("storage failed")
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "value"))
	defer cancel()

	loads := 0
	cached, err := data.NewCache(func(received context.Context, id string) (string, error) {
		loads++
		if received != ctx || received.Value(contextKey{}) != "value" {
			t.Fatal("loader lost request context")
		}

		if loads == 1 {
			return "", fmt.Errorf("load: %w", failure)
		}

		if id == "cancel" {
			cancel()
		}

		return "Ada", nil
	}, data.CacheConfig[string]{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cached.Get(ctx, "42"); !errors.Is(err, failure) {
		t.Fatalf("lost storage error: %v", err)
	}

	for range 2 {
		if _, err := cached.Get(ctx, "42"); err != nil {
			t.Fatal(err)
		}
	}

	if loads != 2 {
		t.Fatalf("errors were cached or hits missed: %d loads", loads)
	}

	if _, err := cached.Get(ctx, "cancel"); !errors.Is(err, context.Canceled) {
		t.Fatalf("returned canceled storage result: %v", err)
	}

	if _, err := cached.Get(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request used cache: %v", err)
	}

	if err := cached.Invalidate(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("invalidation lost cancellation: %v", err)
	}

	if err := cached.InvalidateAll(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("scope invalidation lost cancellation: %v", err)
	}

	// Finish a committed write even when the request was canceled after commit.
	if err := cached.Invalidate(context.WithoutCancel(ctx), "42"); err != nil {
		t.Fatal(err)
	}

	if loads != 3 {
		t.Fatalf("canceled request called storage: %d loads", loads)
	}
}

func TestCacheScopeIsolationAndInvalidation(t *testing.T) {
	type tenantKey struct{}
	missing := errors.New("missing tenant")
	loads := make(map[string]int)
	cached, err := data.NewCache(func(ctx context.Context, id string) (string, error) {
		tenant := ctx.Value(tenantKey{}).(string)
		loads[tenant+id]++
		return tenant, nil
	}, data.CacheConfig[string]{
		TTL: time.Minute,
		Scope: func(ctx context.Context) (string, error) {
			tenant, ok := ctx.Value(tenantKey{}).(string)
			if !ok {
				return "", missing
			}

			return tenant, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	first := context.WithValue(context.Background(), tenantKey{}, "first")
	second := context.WithValue(context.Background(), tenantKey{}, "second")
	read := func() {
		t.Helper()
		for _, ctx := range []context.Context{first, second} {
			for _, id := range []string{"1", "2"} {
				value, err := cached.Get(ctx, id)
				if err != nil || value != ctx.Value(tenantKey{}) {
					t.Fatalf("crossed tenant boundary: %q, %v", value, err)
				}
			}
		}
	}

	read()
	if err := cached.Invalidate(first, "1", "missing"); err != nil {
		t.Fatal(err)
	}
	read()
	if loads["first1"] != 2 || loads["first2"] != 1 || loads["second1"] != 1 || loads["second2"] != 1 {
		t.Fatalf("incorrect per-ID invalidation: %v", loads)
	}

	if err := cached.InvalidateAll(first); err != nil {
		t.Fatal(err)
	}
	read()
	if loads["first1"] != 3 || loads["first2"] != 2 || loads["second1"] != 1 || loads["second2"] != 1 {
		t.Fatalf("incorrect scope invalidation: %v", loads)
	}

	ctx := context.Background()
	if _, err := cached.Get(ctx, "1"); !errors.Is(err, missing) {
		t.Fatalf("missing scope was accepted: %v", err)
	}

	if err := cached.Invalidate(ctx, "1"); !errors.Is(err, missing) {
		t.Fatalf("invalidation lost scope error: %v", err)
	}

	if err := cached.InvalidateAll(ctx); !errors.Is(err, missing) {
		t.Fatalf("scope invalidation lost scope error: %v", err)
	}
}

func TestInvalidationFencesInFlightReads(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			t.Cleanup(unblock)
			var loads atomic.Int64
			var version atomic.Int64
			cached, err := data.NewCache(func(context.Context, string) (int64, error) {
				value := version.Load()
				if loads.Add(1) == 1 {
					close(started)
					<-release
				}

				return value, nil
			}, data.CacheConfig[int64]{TTL: time.Minute})
			if err != nil {
				t.Fatal(err)
			}

			done := make(chan error, 1)
			go func() {
				_, err := cached.Get(context.Background(), "42")
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("load did not start")
			}

			version.Add(1)
			if all {
				err = cached.InvalidateAll(context.Background())
			} else {
				err = cached.Invalidate(context.Background(), "42")
			}
			if err != nil {
				t.Fatal(err)
			}

			unblock()
			if err := <-done; err != nil {
				t.Fatal(err)
			}

			for range 2 {
				value, err := cached.Get(context.Background(), "42")
				if err != nil || value != 1 {
					t.Fatalf("stale load repopulated cache: %d, %v", value, err)
				}
			}

			if loads.Load() != 2 {
				t.Fatalf("unexpected loads: %d", loads.Load())
			}
		})
	}
}

func TestCacheCloneAndConcurrentUse(t *testing.T) {
	cached, err := data.NewCache(func(context.Context, int) ([]string, error) {
		return []string{"reader"}, nil
	}, data.CacheConfig[[]string]{
		TTL:        time.Minute,
		MaxEntries: 8,
		Clone:      slices.Clone[[]string],
	})
	if err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for worker := range 12 {
		workers.Go(func() {
			for iteration := range 50 {
				id := (worker + iteration) % 16
				value, err := cached.Get(context.Background(), id)
				if err != nil || value[0] != "reader" {
					t.Errorf("mutation leaked into cache: %v, %v", value, err)
					return
				}

				value[0] = "admin"
				if iteration%2 == 0 {
					err = cached.Invalidate(context.Background(), id)
				} else {
					err = cached.InvalidateAll(context.Background())
				}
				if err != nil {
					t.Error(err)
				}
			}
		})
	}

	workers.Wait()
}

func TestCanceledLoadIsRetriedAndCommittedInvalidationStillRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loads := 0
	cached, err := data.NewCache(func(context.Context, string) (int, error) {
		loads++
		if loads == 1 {
			cancel()
		}

		return loads, nil
	}, data.CacheConfig[int]{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cached.Get(ctx, "old"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load succeeded: %v", err)
	}
	for _, id := range []string{"old", "new"} {
		if _, err := cached.Get(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if loads != 3 {
		t.Fatalf("canceled load was cached: %d loads", loads)
	}

	// A commit can change the ID and finish after its request was canceled.
	if err := cached.Invalidate(context.WithoutCancel(ctx), "old", "new"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old", "new"} {
		if _, err := cached.Get(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if loads != 5 {
		t.Fatalf("post-commit invalidation missed an ID: %d loads", loads)
	}
}
