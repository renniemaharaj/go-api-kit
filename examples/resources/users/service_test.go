package users

import (
	"context"
	"errors"
	"testing"
)

func TestServiceValidationPreservesStoredUser(t *testing.T) {
	repository := newRepository(User{
		ID:   "42",
		Name: "Ada",
	})
	service := newService(repository)

	ctx := context.Background()
	if _, err := service.Save(ctx, User{
		ID:   "42",
		Name: " ",
	}); !errors.Is(err, errInvalidUser) {
		t.Fatalf("invalid user was accepted: %v", err)
	}

	user, err := service.Get(ctx, "42")
	if err != nil || user.Name != "Ada" {
		t.Fatalf("invalid write changed the user: %#v, %v", user, err)
	}
}

func TestServiceRejectsCanceledRequestsBeforeCallingRepository(t *testing.T) {
	repository := &observedRepository{}
	service := newService(repository)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := service.Get(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get lost cancellation: %v", err)
	}

	if _, err := service.Save(ctx, User{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save must check cancellation before validation: %v", err)
	}

	if err := service.Delete(ctx, "42"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete lost cancellation: %v", err)
	}

	if repository.calls != 0 {
		t.Fatalf("canceled requests reached storage %d times", repository.calls)
	}
}

func TestServiceRejectsReadsCanceledDuringStorageAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := newService(&observedRepository{cancel: cancel})
	user, err := service.Get(ctx, "42")
	if !errors.Is(err, context.Canceled) || user != (User{}) {
		t.Fatalf("returned a canceled read: %#v, %v", user, err)
	}
}

type observedRepository struct {
	calls  int
	cancel context.CancelFunc
}

func (r *observedRepository) Get(context.Context, string) (User, error) {
	r.calls++
	if r.cancel != nil {
		r.cancel()
	}

	return User{ID: "42"}, nil
}

func (r *observedRepository) Save(_ context.Context, user User) (User, error) {
	r.calls++
	return user, nil
}

func (r *observedRepository) Delete(context.Context, string) error {
	r.calls++
	return nil
}
