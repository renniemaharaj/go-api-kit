package kit

import "net/http"

// HeaderAPI provides request and response header operations.
type HeaderAPI struct {
	w http.ResponseWriter
	r *http.Request
}

// Get returns the first value of a request header.
func (h HeaderAPI) Get(name string) string {
	return h.r.Header.Get(name)
}

// Values returns all values for a request header.
func (h HeaderAPI) Values(name string) []string {
	return h.r.Header.Values(name)
}

// Set sets a response header.
func (h HeaderAPI) Set(name, value string) {
	h.w.Header().Set(name, value)
}

// Add adds a response header value.
func (h HeaderAPI) Add(name, value string) {
	h.w.Header().Add(name, value)
}

// Del removes a response header.
func (h HeaderAPI) Del(name string) {
	h.w.Header().Del(name)
}
