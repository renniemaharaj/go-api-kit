package users

import (
	"context"
	"sync"

	"github.com/renniemaharaj/go-api-kit/data"
)

// repository is the example storage implementation.
// A database implementation can satisfy the same data.Repository contract.
type repository struct {
	mu    sync.RWMutex
	users map[string]User
}

var _ data.Repository[User, string] = (*repository)(nil)

func newRepository(users ...User) *repository {
	store := &repository{users: make(map[string]User, len(users))}
	for _, user := range users {
		store.users[user.ID] = user
	}

	return store
}

func (r *repository) Get(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return User{}, data.ErrNotFound
	}

	return user, nil
}

func (r *repository) Save(ctx context.Context, user User) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.users[user.ID] = user

	return user, nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return data.ErrNotFound
	}

	delete(r.users, id)
	return nil
}
