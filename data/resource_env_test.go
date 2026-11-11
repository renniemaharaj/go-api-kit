package data_test

import (
	"context"
	"testing"

	"github.com/renniemaharaj/go-api-kit/data"
)

func TestResourceSpecificEnvironmentOverridesCacheDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("RESOURCE_CACHE_TTL", "1h")
	t.Setenv("RESOURCE_CACHE_MAX_ENTRIES", "2")
	t.Setenv("USERS_CACHE_TTL", "0s")

	loads := 0
	resource, err := data.NewResourceFromEnv(data.ResourceConfig[string, string]{
		Name: "users",
		Load: func(context.Context, string) (string, error) {
			loads++
			return "Ada", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if _, err := resource.Get(context.Background(), "42"); err != nil {
			t.Fatal(err)
		}
	}

	if loads != 2 {
		t.Fatalf("resource-specific cache override was ignored: %d loads", loads)
	}

	t.Setenv("USERS_CACHE_TTL", "invalid")
	if _, err := data.NewResourceFromEnv(data.ResourceConfig[string, string]{Name: "users"}); err == nil {
		t.Fatal("invalid resource cache settings were accepted")
	}
}
