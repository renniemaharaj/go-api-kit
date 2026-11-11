package data_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
)

type exampleUser struct {
	ID   string
	Name string
}

type exampleUserRepository struct {
	loads atomic.Int64
}

var _ data.Repository[exampleUser, string] = (*exampleUserRepository)(nil)

func (r *exampleUserRepository) Get(ctx context.Context, id string) (exampleUser, error) {
	if err := ctx.Err(); err != nil {
		return exampleUser{}, err
	}

	r.loads.Add(1)
	return exampleUser{
		ID:   id,
		Name: "Ada",
	}, nil
}

func (r *exampleUserRepository) Save(context.Context, exampleUser) (exampleUser, error) {
	return exampleUser{}, data.ErrUnsupported
}

func (r *exampleUserRepository) Delete(context.Context, string) error {
	return data.ErrUnsupported
}

func exampleUserKey(user exampleUser) string {
	return user.ID
}

func ExampleNewResource() {
	repository := &exampleUserRepository{}
	users, err := data.NewResource(data.ResourceConfig[exampleUser, string]{
		Name:       "users",
		Repository: repository,
		Key:        exampleUserKey,
		Cache: data.CachePolicy{
			TTL: time.Minute,
		},
	})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	for range 2 {
		user, err := users.Get(ctx, "42")
		if err != nil {
			panic(err)
		}

		fmt.Println(user.Name)
	}

	fmt.Println("Storage calls:", repository.loads.Load())

	if err := users.Invalidate(ctx, "42"); err != nil {
		panic(err)
	}

	if _, err := users.Get(ctx, "42"); err != nil {
		panic(err)
	}

	fmt.Println("After invalidation:", repository.loads.Load())

	// Output:
	// Ada
	// Ada
	// Storage calls: 1
	// After invalidation: 2
}
