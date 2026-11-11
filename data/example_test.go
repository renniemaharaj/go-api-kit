package data_test

import (
	"context"
	"fmt"
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
)

func ExampleNewCache() {
	loads := 0
	users, err := data.NewCache(func(ctx context.Context, id string) (string, error) {
		loads++
		return "Ada", nil
	}, data.CacheConfig[string]{TTL: time.Minute})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	for range 2 {
		name, err := users.Get(ctx, "42")
		if err != nil {
			panic(err)
		}

		fmt.Println(name)
	}

	fmt.Println("Storage calls:", loads)
	if err := users.Invalidate(ctx, "42"); err != nil {
		panic(err)
	}

	if _, err := users.Get(ctx, "42"); err != nil {
		panic(err)
	}

	fmt.Println("After invalidation:", loads)
	// Output:
	// Ada
	// Ada
	// Storage calls: 1
	// After invalidation: 2
}
