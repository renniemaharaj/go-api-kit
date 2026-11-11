// Package health demonstrates read-only repository, service, and handler wiring.
package health

import (
	"time"

	"github.com/renniemaharaj/go-api-kit/data"
)

type Snapshot struct {
	StartedAt time.Time
}

type Status struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// New wires a process liveness resource with an immutable startup snapshot.
func New() data.Definition {
	storage := newRepository()
	checks := &service{repository: storage}
	handlers := handler{service: checks}

	return data.Definition{
		Name:   "health",
		Path:   "/health",
		Routes: handlers.Routes(),
	}
}
