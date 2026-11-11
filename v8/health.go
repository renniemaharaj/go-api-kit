package v8

import (
	"log"
	"net/http"
	"time"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func healthHandler() kit.HandlerFunc {
	started := time.Now()

	return func(h kit.Handle) {
		code := http.StatusOK
		status := struct {
			Status        string `json:"status"`
			UptimeSeconds int64  `json:"uptime_seconds"`
		}{
			Status:        "ok",
			UptimeSeconds: max(0, int64(time.Since(started)/time.Second)),
		}
		if h.Context().Err() != nil {
			code = http.StatusServiceUnavailable
			status.Status = "unavailable"
			status.UptimeSeconds = 0
		}

		h.Writer().Header().Set("Cache-Control", "no-store")
		if h.Request().Method == http.MethodHead {
			h.Writer().Header().Set("Content-Type", "application/json; charset=utf-8")
			h.Status(code)
			return
		}

		if err := h.JSON(code, status); err != nil {
			log.Printf("write health response: %v", err)
		}
	}
}
