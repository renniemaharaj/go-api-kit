package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/repository"
)

type testRecord struct {
	ID   string
	Name string
}

func recordKey(value testRecord) string {
	return value.ID
}

func newTestResource(t *testing.T, config ResourceConfig[testRecord, string]) *Resource[testRecord, string] {
	t.Helper()

	if config.Name == "" {
		config.Name = "users"
	}

	resource, err := NewResource(config)
	if err != nil {
		t.Fatal(err)
	}

	return resource
}

func TestResourceValidatesConfiguration(t *testing.T) {
	load := func(context.Context, string) (testRecord, error) {
		return testRecord{}, nil
	}
	save := func(_ context.Context, value testRecord) (testRecord, error) {
		return value, nil
	}

	tests := []struct {
		name   string
		config ResourceConfig[testRecord, string]
		want   string
	}{
		{
			name: "name",
			config: ResourceConfig[testRecord, string]{
				Name: " ",
				Load: load,
			},
			want: "name is required",
		},
		{
			name:   "load",
			config: ResourceConfig[testRecord, string]{Name: "users"},
			want:   "Load or Repository is required",
		},
		{
			name: "conflicting storage",
			config: ResourceConfig[testRecord, string]{
				Name:       "users",
				Load:       load,
				Repository: repository.Functions[testRecord, string]{Load: load},
			},
			want: "either Repository or storage functions",
		},
		{
			name: "negative TTL",
			config: ResourceConfig[testRecord, string]{
				Name:  "users",
				Load:  load,
				Cache: CachePolicy{TTL: -time.Second},
			},
			want: "cannot be negative",
		},
		{
			name: "negative capacity",
			config: ResourceConfig[testRecord, string]{
				Name:  "users",
				Load:  load,
				Cache: CachePolicy{MaxEntries: -1},
			},
			want: "cannot be negative",
		},
		{
			name: "cached save without key",
			config: ResourceConfig[testRecord, string]{
				Name:  "users",
				Load:  load,
				Save:  save,
				Cache: CachePolicy{TTL: time.Minute},
			},
			want: "Key is required",
		},
		{
			name: "cached repository without key",
			config: ResourceConfig[testRecord, string]{
				Name:       "users",
				Repository: repository.Functions[testRecord, string]{Load: load},
				Cache:      CachePolicy{TTL: time.Minute},
			},
			want: "Key is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewResource(test.config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestCachingIsOptIn(t *testing.T) {
	for _, ttl := range []time.Duration{
		0,
		time.Minute,
	} {
		t.Run(ttl.String(), func(t *testing.T) {
			loads := 0
			resource := newTestResource(t, ResourceConfig[testRecord, string]{
				Load: func(_ context.Context, id string) (testRecord, error) {
					loads++

					return testRecord{ID: id}, nil
				},
				Cache: CachePolicy{TTL: ttl},
			})

			ctx := context.Background()
			for range 2 {
				value, err := resource.Get(ctx, "42")
				if err != nil || value.ID != "42" {
					t.Fatalf("got %#v, %v", value, err)
				}
			}

			wantLoads := 2
			if ttl > 0 {
				wantLoads = 1
			}

			if loads != wantLoads {
				t.Fatalf("loaded %d times, want %d", loads, wantLoads)
			}

			if _, err := resource.Save(ctx, testRecord{}); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("unsupported Save: %v", err)
			}

			if err := resource.Delete(ctx, "42"); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("unsupported Delete: %v", err)
			}

			if err := resource.Invalidate(ctx, "42"); err != nil {
				t.Fatal(err)
			}

			if err := resource.InvalidateAll(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServiceRulesRunBeforeCacheHitsAndWrites(t *testing.T) {
	ctx := context.Background()
	denied := errors.New("permission denied")
	allow := true
	checks := 0
	loads := 0
	writes := 0
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Key: recordKey,
		Load: func(_ context.Context, id string) (testRecord, error) {
			loads++

			return testRecord{ID: id}, nil
		},
		Save: func(_ context.Context, value testRecord) (testRecord, error) {
			writes++

			return value, nil
		},
		Delete: func(context.Context, string) error {
			writes++

			return nil
		},
		BeforeGet: func(context.Context, string) error {
			checks++
			if !allow {
				return denied
			}

			return nil
		},
		BeforeSave: func(context.Context, testRecord) error {
			return denied
		},
		BeforeDelete: func(context.Context, string) error {
			return denied
		},
		Cache: CachePolicy{TTL: time.Minute},
	})

	for range 2 {
		if _, err := resource.Get(ctx, "42"); err != nil {
			t.Fatal(err)
		}
	}

	allow = false
	if _, err := resource.Get(ctx, "42"); !errors.Is(err, denied) {
		t.Fatalf("cached read bypassed rules: %v", err)
	}

	if _, err := resource.Save(ctx, testRecord{ID: "42"}); !errors.Is(err, denied) {
		t.Fatalf("Save bypassed rules: %v", err)
	}

	if err := resource.Delete(ctx, "42"); !errors.Is(err, denied) {
		t.Fatalf("Delete bypassed rules: %v", err)
	}

	if checks != 3 || loads != 1 || writes != 0 {
		t.Fatalf("checks=%d loads=%d writes=%d", checks, loads, writes)
	}
}

func TestLoadErrorsAreNotCached(t *testing.T) {
	loads := 0
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Load: func(_ context.Context, id string) (testRecord, error) {
			loads++
			if loads == 1 {
				return testRecord{}, ErrNotFound
			}

			return testRecord{ID: id}, nil
		},
		Cache: CachePolicy{TTL: time.Minute},
	})

	ctx := context.Background()
	if _, err := resource.Get(ctx, "42"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected wrapped ErrNotFound, got %v", err)
	}

	for range 2 {
		if _, err := resource.Get(ctx, "42"); err != nil {
			t.Fatal(err)
		}
	}

	if loads != 2 {
		t.Fatalf("loaded %d times, want 2", loads)
	}
}

func TestExplicitInvalidationAndScopes(t *testing.T) {
	type tenantKey struct{}
	scopeError := errors.New("missing tenant")
	loads := make(map[string]int)
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Load: func(ctx context.Context, id string) (testRecord, error) {
			tenant := ctx.Value(tenantKey{}).(string)
			loads[tenant+id]++

			return testRecord{
				ID:   id,
				Name: tenant,
			}, nil
		},
		Scope: func(ctx context.Context) (string, error) {
			tenant, ok := ctx.Value(tenantKey{}).(string)
			if !ok {
				return "", scopeError
			}

			return tenant, nil
		},
		Cache: CachePolicy{TTL: time.Minute},
	})

	first := context.WithValue(context.Background(), tenantKey{}, "first")
	second := context.WithValue(context.Background(), tenantKey{}, "second")
	get := func(ctx context.Context, id string) {
		t.Helper()

		value, err := resource.Get(ctx, id)
		if err != nil || value.Name != ctx.Value(tenantKey{}) {
			t.Fatalf("crossed tenant boundary: %#v, %v", value, err)
		}
	}

	get(first, "1")
	get(first, "2")
	get(second, "1")
	if err := resource.Invalidate(first, "1", "missing"); err != nil {
		t.Fatal(err)
	}

	get(first, "1")
	get(first, "2")
	get(second, "1")
	if loads["first1"] != 2 || loads["first2"] != 1 || loads["second1"] != 1 {
		t.Fatalf("incorrect item invalidation: %v", loads)
	}

	if err := resource.InvalidateAll(first); err != nil {
		t.Fatal(err)
	}

	get(first, "1")
	get(first, "2")
	get(second, "1")
	if loads["first1"] != 3 || loads["first2"] != 2 || loads["second1"] != 1 {
		t.Fatalf("incorrect scope invalidation: %v", loads)
	}

	ctx := context.Background()
	if _, err := resource.Get(ctx, "1"); !errors.Is(err, scopeError) {
		t.Fatalf("Get should reject missing scope: %v", err)
	}

	if err := resource.Invalidate(ctx, "1"); !errors.Is(err, scopeError) {
		t.Fatalf("Invalidate should reject missing scope: %v", err)
	}

	if err := resource.InvalidateAll(ctx); !errors.Is(err, scopeError) {
		t.Fatalf("InvalidateAll should reject missing scope: %v", err)
	}
}

func TestRepositoryWritesInvalidateOnlyAfterSuccess(t *testing.T) {
	ctx := context.Background()
	writeError := errors.New("write failed")
	fail := true
	loads := 0
	stored := testRecord{
		ID:   "42",
		Name: "old",
	}
	existing := repository.Functions[testRecord, string]{
		Load: func(_ context.Context, id string) (testRecord, error) {
			loads++

			return testRecord{
				ID:   id,
				Name: stored.Name,
			}, nil
		},
		Store: func(_ context.Context, value testRecord) (testRecord, error) {
			if fail {
				return testRecord{}, writeError
			}

			stored = value

			return value, nil
		},
		Remove: func(context.Context, string) error {
			if fail {
				return writeError
			}

			stored.Name = "deleted"

			return nil
		},
	}
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Repository: existing,
		Key:        recordKey,
		Cache:      CachePolicy{TTL: time.Minute},
	})

	if _, err := resource.Get(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	if _, err := resource.Save(ctx, testRecord{ID: "42"}); !errors.Is(err, writeError) {
		t.Fatalf("expected Save failure: %v", err)
	}

	if err := resource.Delete(ctx, "42"); !errors.Is(err, writeError) {
		t.Fatalf("expected Delete failure: %v", err)
	}

	value, err := resource.Get(ctx, "42")
	if err != nil || value.Name != "old" || loads != 1 {
		t.Fatalf("failed write evicted cache: %#v, %v, loads=%d", value, err, loads)
	}

	fail = false
	// A custom service can use Repository directly and keep caching behavior.
	repository := resource.Repository()
	if _, err := repository.Save(ctx, testRecord{
		ID:   "42",
		Name: "new",
	}); err != nil {
		t.Fatal(err)
	}

	value, err = resource.Get(ctx, "42")
	if err != nil || value.Name != "new" || loads != 2 {
		t.Fatalf("Save did not invalidate: %#v, %v, loads=%d", value, err, loads)
	}

	if err := repository.Delete(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	value, err = resource.Get(ctx, "42")
	if err != nil || value.Name != "deleted" || loads != 3 {
		t.Fatalf("Delete did not invalidate: %#v, %v, loads=%d", value, err, loads)
	}
}

func TestSaveInvalidatesOriginalAndReturnedIDs(t *testing.T) {
	ctx := context.Background()
	loads := 0
	resource, err := NewResource(ResourceConfig[*testRecord, string]{
		Name: "users",
		Key: func(value *testRecord) string {
			return value.ID
		},
		Load: func(_ context.Context, id string) (*testRecord, error) {
			loads++

			return &testRecord{ID: id}, nil
		},
		Save: func(_ context.Context, value *testRecord) (*testRecord, error) {
			value.ID = "new"

			return value, nil
		},
		Clone: func(value *testRecord) *testRecord {
			copy := *value

			return &copy
		},
		Cache: CachePolicy{TTL: time.Minute},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{
		"old",
		"new",
	} {
		if _, err := resource.Get(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := resource.Save(ctx, &testRecord{ID: "old"}); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{
		"old",
		"new",
	} {
		if _, err := resource.Get(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	if loads != 4 {
		t.Fatalf("loaded %d times, want both IDs invalidated", loads)
	}
}

func TestCanceledRequestsDoNotUseCacheOrCallHooks(t *testing.T) {
	hooks := 0
	loads := 0
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Load: func(_ context.Context, id string) (testRecord, error) {
			loads++

			return testRecord{ID: id}, nil
		},
		BeforeGet: func(context.Context, string) error {
			hooks++

			return nil
		},
		Cache: CachePolicy{TTL: time.Minute},
	})
	if _, err := resource.Get(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resource.Get(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get: %v", err)
	}

	if _, err := resource.Save(ctx, testRecord{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save: %v", err)
	}

	if err := resource.Delete(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete: %v", err)
	}

	if err := resource.Invalidate(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Invalidate: %v", err)
	}

	if err := resource.InvalidateAll(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("InvalidateAll: %v", err)
	}

	if hooks != 1 || loads != 1 {
		t.Fatalf("canceled call ran work: hooks=%d loads=%d", hooks, loads)
	}
}

func TestSuccessfulSaveInvalidatesDespiteCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loads := 0
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Key: recordKey,
		Load: func(_ context.Context, id string) (testRecord, error) {
			loads++

			return testRecord{ID: id}, nil
		},
		Save: func(_ context.Context, value testRecord) (testRecord, error) {
			cancel()
			return value, nil
		},
		Cache: CachePolicy{TTL: time.Minute},
	})

	if _, err := resource.Get(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	if _, err := resource.Save(ctx, testRecord{ID: "42"}); err != nil {
		t.Fatal(err)
	}

	if _, err := resource.Get(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}

	if loads != 2 {
		t.Fatalf("successful save did not invalidate after cancellation: loads=%d", loads)
	}
}

func TestInvalidationFencesInFlightReads(t *testing.T) {
	for _, operation := range []string{
		"invalidate",
		"invalidate all",
		"save",
		"delete",
	} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			started := make(chan struct{})
			release := make(chan struct{})
			unblock := sync.OnceFunc(func() {
				close(release)
			})
			t.Cleanup(unblock)

			var loads atomic.Int64

			var version atomic.Int64
			resource := newTestResource(t, ResourceConfig[testRecord, string]{
				Key: recordKey,
				Load: func(_ context.Context, id string) (testRecord, error) {
					name := fmt.Sprint(version.Load())
					if loads.Add(1) == 1 {
						close(started)
						<-release
					}

					return testRecord{
						ID:   id,
						Name: name,
					}, nil
				},
				Save: func(_ context.Context, value testRecord) (testRecord, error) {
					version.Add(1)
					return value, nil
				},
				Delete: func(context.Context, string) error {
					version.Add(1)
					return nil
				},
				Cache: CachePolicy{TTL: time.Minute},
			})

			done := make(chan error, 1)
			go func() {
				_, err := resource.Get(ctx, "42")
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("load did not start")
			}

			var err error
			switch operation {
			case "invalidate":
				version.Add(1)
				err = resource.Invalidate(ctx, "42")
			case "invalidate all":
				version.Add(1)
				err = resource.InvalidateAll(ctx)
			case "save":
				_, err = resource.Save(ctx, testRecord{ID: "42"})
			case "delete":
				err = resource.Delete(ctx, "42")
			}

			if err != nil {
				t.Fatal(err)
			}

			unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("load did not finish")
			}

			for range 2 {
				value, err := resource.Get(ctx, "42")
				if err != nil || value.Name != "1" {
					t.Fatalf("stale read repopulated cache: %#v, %v", value, err)
				}
			}

			if loads.Load() != 2 {
				t.Fatalf("loads=%d, want 2", loads.Load())
			}
		})
	}
}

func TestCloneProtectsCachedSlices(t *testing.T) {
	resource, err := NewResource(ResourceConfig[[]string, string]{
		Name: "roles",
		Load: func(context.Context, string) ([]string, error) {
			return []string{"reader"}, nil
		},
		Clone: func(value []string) []string {
			return append([]string(nil), value...)
		},
		Cache: CachePolicy{TTL: time.Minute},
	})
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		roles, err := resource.Get(context.Background(), "42")
		if err != nil || roles[0] != "reader" {
			t.Fatalf("mutation leaked into cache: %v, %v", roles, err)
		}

		roles[0] = "admin"
	}
}

func TestResourceConcurrentUse(t *testing.T) {
	var version atomic.Int64
	resource := newTestResource(t, ResourceConfig[testRecord, string]{
		Key: recordKey,
		Load: func(_ context.Context, id string) (testRecord, error) {
			return testRecord{
				ID:   id,
				Name: fmt.Sprint(version.Load()),
			}, nil
		},
		Save: func(_ context.Context, value testRecord) (testRecord, error) {
			version.Add(1)
			return value, nil
		},
		Delete: func(context.Context, string) error {
			version.Add(1)
			return nil
		},
		Cache: CachePolicy{
			TTL:        time.Minute,
			MaxEntries: 8,
		},
	})

	var workers sync.WaitGroup
	for worker := range 12 {
		workers.Go(func() {
			ctx := context.Background()
			for iteration := range 50 {
				id := fmt.Sprint((worker + iteration) % 16)

				var err error
				switch iteration % 5 {
				case 0:
					_, err = resource.Get(ctx, id)
				case 1:
					_, err = resource.Save(ctx, testRecord{ID: id})
				case 2:
					err = resource.Delete(ctx, id)
				case 3:
					err = resource.Invalidate(ctx, id)
				case 4:
					err = resource.InvalidateAll(ctx)
				}

				if err != nil {
					t.Error(err)
					return
				}
			}
		})
	}

	workers.Wait()
}
