package data

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// A separate routing tree validates patterns without running app middleware.
func validateRoutes(routes []resourceRoute) error {
	probe := chi.NewRouter()
	for index, route := range routes {
		if err := probeRoute(probe, route, routeMarker(index)); err != nil {
			return err
		}
	}

	registered := make(map[int]bool, len(routes))
	err := chi.Walk(probe, func(_ string, _ string, handler http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[int(handler.(routeMarker))] = true
		return nil
	})
	if err != nil {
		return err
	}

	for index, route := range routes {
		if !registered[index] {
			return fmt.Errorf("data: %s: conflicting route %s %s", route.resource, route.method, route.path)
		}
	}

	return nil
}

func probeRoute(router chi.Router, route resourceRoute, marker routeMarker) (err error) {
	defer func() {
		if problem := recover(); problem != nil {
			err = fmt.Errorf("data: %s: invalid route %s %s: %v", route.resource, route.method, route.path, problem)
		}
	}()

	router.Method(route.method, route.path, marker)
	return nil
}

type routeMarker int

func (routeMarker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
