package health

import (
	"context"
	"log"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

// Service is the health operation consumed by the HTTP handler.
type Service interface {
	Check(context.Context) (Status, error)
}

type handler struct {
	service Service
}

func (h handler) Routes() []data.Route {
	return []data.Route{
		{
			Method:  http.MethodGet,
			Handler: h.Check,
		},
		{
			Method:  http.MethodHead,
			Handler: h.Check,
		},
	}
}

func (h handler) Check(request kit.Handle) {
	result, err := h.service.Check(request.Context())
	code := http.StatusOK
	if err != nil {
		log.Printf("health check: %v", err)
		code = http.StatusServiceUnavailable
		result = Status{Status: "unavailable"}
	}

	request.Writer().Header().Set("Cache-Control", "no-store")
	if request.Request().Method == http.MethodHead {
		request.Writer().Header().Set("Content-Type", "application/json; charset=utf-8")
		request.Status(code)
		return
	}

	if err := request.JSON(code, result); err != nil {
		log.Printf("write health response: %v", err)
	}
}
