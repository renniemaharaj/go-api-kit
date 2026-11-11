package users

import (
	"context"
	"testing"
)

func TestResourceInvalidatesExternalWrites(t *testing.T) {
	ctx := context.Background()
	repository := newRepository(User{
		ID:   "42",
		Name: "Ada",
	})
	resource, err := newResource(repository)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := resource.Get(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.Save(ctx, User{
		ID:   "42",
		Name: "Lin",
	}); err != nil {
		t.Fatal(err)
	}

	user, err := resource.Get(ctx, "42")
	if err != nil || user.Name != "Ada" {
		t.Fatalf("external write bypassed cache: %#v, %v", user, err)
	}

	if err := resource.Invalidate(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	user, err = resource.Get(ctx, "42")
	if err != nil || user.Name != "Lin" {
		t.Fatalf("invalidation did not refresh the user: %#v, %v", user, err)
	}
}
