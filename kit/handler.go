package kit

import (
	"net/http"
)

type Handle struct {
	w http.ResponseWriter
	r *http.Request
}

func NewHandler(fn func(h Handle)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(Handle{
			w: w,
			r: r,
		})
	}
}

func (h Handle) Writer() http.ResponseWriter {
	return h.w
}

func (h Handle) Request() *http.Request {
	return h.r
}

func (h Handle) Path() string {
	return h.r.URL.Path
}

func (h Handle) Status(code int) {
	h.w.WriteHeader(code)
}
