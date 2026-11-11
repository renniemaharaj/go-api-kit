package users

import (
	"context"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Resource connects the user service, storage cache, and HTTP definition.
type Resource struct {
	service *service
	records *data.Resource[User, string]
}

var _ data.Repository[User, string] = (*Resource)(nil)

// New creates the user resource with its service and in-memory repository.
func New(records ...User) (*Resource, error) {
	return newResource(newRepository(records...))
}

func newResource(storage data.Repository[User, string]) (*Resource, error) {
	records, err := data.NewResourceFromEnv(data.ResourceConfig[User, string]{
		Name:       "users",
		Repository: storage,
		Key:        key,
	})
	if err != nil {
		return nil, err
	}

	return &Resource{
		service: newService(records.Repository()),
		records: records,
	}, nil
}

func (r *Resource) Get(ctx context.Context, id string) (User, error) {
	return r.service.Get(ctx, id)
}

func (r *Resource) Save(ctx context.Context, user User) (User, error) {
	return r.service.Save(ctx, user)
}

func (r *Resource) Delete(ctx context.Context, id string) error {
	return r.service.Delete(ctx, id)
}

func (r *Resource) Invalidate(ctx context.Context, ids ...string) error {
	return r.records.Invalidate(ctx, ids...)
}

// Definition declares the resource's endpoints and shared access policies.
func (r *Resource) Definition(with ...kit.Middleware) data.Definition {
	handlers := handler{service: r.service}

	return data.Definition{
		Name:       "users",
		Path:       "/users",
		Middleware: with,
		Routes:     handlers.Routes(),
		Errors: kit.ErrorConfig{
			Rules: []kit.ErrorRule{
				{
					Error:   data.ErrNotFound,
					Status:  http.StatusNotFound,
					Message: "user not found",
				},
				{
					Error:   errInvalidUser,
					Status:  http.StatusBadRequest,
					Message: errInvalidUser.Error(),
				},
			},
			InternalMessage: "could not complete user operation",
		},
	}
}

func key(user User) string {
	return user.ID
}
