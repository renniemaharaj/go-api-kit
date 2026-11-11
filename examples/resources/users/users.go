package users

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
)

var (
	errNotFound    = errors.New("user not found")
	errInvalidUser = errors.New("user ID and name are required")
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Users keeps storage and business rules together for this small example.
// Larger applications can inject database loading functions into the same cache.
type Users struct {
	mu     sync.RWMutex
	stored map[string]User
	*data.Cache[User, string]
}

// New explicitly opts this example into a one-minute cache.
func New(records ...User) (*Users, error) {
	users := &Users{stored: make(map[string]User, len(records))}
	for _, user := range records {
		users.stored[user.ID] = user
	}

	var err error
	users.Cache, err = data.NewCache(users.load, data.CacheConfig[User]{
		TTL:        time.Minute,
		MaxEntries: 256,
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (u *Users) load(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	u.mu.RLock()
	defer u.mu.RUnlock()

	user, ok := u.stored[id]
	if !ok {
		return User{}, errNotFound
	}

	return user, nil
}

func (u *Users) Save(ctx context.Context, user User) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	if user.ID == "" || strings.TrimSpace(user.Name) == "" {
		return User{}, errInvalidUser
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return User{}, err
	}

	u.stored[user.ID] = user
	// A committed write must invalidate even if its request was canceled.
	err := u.Cache.Invalidate(context.WithoutCancel(ctx), user.ID)
	return user, err
}

func (u *Users) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, ok := u.stored[id]; !ok {
		return errNotFound
	}

	delete(u.stored, id)
	return u.Cache.Invalidate(context.WithoutCancel(ctx), id)
}
