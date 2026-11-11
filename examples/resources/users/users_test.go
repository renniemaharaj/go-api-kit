package users

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestWritesValidateAndInvalidate(t *testing.T) {
	u, err := New(User{ID: "42", Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	assertName := func(want string) {
		t.Helper()
		user, err := u.Get(ctx, "42")
		if err != nil || user.Name != want {
			t.Fatalf("got %#v, %v; want %s", user, err, want)
		}
	}

	assertName("Ada")
	if _, err := u.Save(ctx, User{ID: "42", Name: " "}); !errors.Is(err, errInvalidUser) {
		t.Fatalf("invalid write was accepted: %v", err)
	}
	assertName("Ada")

	if _, err := u.Save(ctx, User{ID: "42", Name: "Grace"}); err != nil {
		t.Fatal(err)
	}
	assertName("Grace")

	// A storage change outside Save must explicitly invalidate cached reads.
	u.mu.Lock()
	u.stored["42"] = User{ID: "42", Name: "Lin"}
	u.mu.Unlock()
	assertName("Grace")
	if err := u.Invalidate(ctx, "42"); err != nil {
		t.Fatal(err)
	}
	assertName("Lin")

	if err := u.Delete(ctx, "42"); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Get(ctx, "42"); !errors.Is(err, errNotFound) {
		t.Fatalf("delete retained cached user: %v", err)
	}
}

func TestCanceledWritesLeaveStorageAndCacheUntouched(t *testing.T) {
	u, err := New(User{ID: "42", Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := u.Get(ctx, "42"); err != nil {
		t.Fatal(err)
	}
	cancel()

	if _, err := u.Save(ctx, User{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save must check cancellation before validation: %v", err)
	}
	if err := u.Delete(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete lost cancellation: %v", err)
	}

	user, err := u.Get(context.Background(), "42")
	if err != nil || user.Name != "Ada" {
		t.Fatalf("canceled write changed user: %#v, %v", user, err)
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			id := fmt.Sprint(worker)
			for iteration := range 30 {
				ctx := context.Background()
				name := fmt.Sprint(iteration)
				if _, err := u.Save(ctx, User{ID: id, Name: name}); err != nil {
					t.Error(err)
					return
				}

				user, err := u.Get(ctx, id)
				if err != nil || user.Name != name {
					t.Errorf("write did not invalidate: %#v, %v", user, err)
					return
				}

				if err := u.Delete(ctx, id); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	workers.Wait()
}
