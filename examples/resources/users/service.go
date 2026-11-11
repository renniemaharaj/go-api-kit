package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/renniemaharaj/go-api-kit/data"
)

var errInvalidUser = errors.New("user ID and name are required")

type service struct {
	repository data.Repository[User, string]
}

func newService(repository data.Repository[User, string]) *service {
	return &service{repository: repository}
}

func (s *service) Get(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, fmt.Errorf("users: get: %w", err)
	}

	user, err := s.repository.Get(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("users: get: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return User{}, fmt.Errorf("users: get: %w", err)
	}

	return user, nil
}

func (s *service) Save(ctx context.Context, user User) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, fmt.Errorf("users: save: %w", err)
	}

	if user.ID == "" || strings.TrimSpace(user.Name) == "" {
		return User{}, fmt.Errorf("users: save: %w", errInvalidUser)
	}

	saved, err := s.repository.Save(ctx, user)
	if err != nil {
		return User{}, fmt.Errorf("users: save: %w", err)
	}

	return saved, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("users: delete: %w", err)
	}

	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("users: delete: %w", err)
	}

	return nil
}
