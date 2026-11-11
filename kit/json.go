package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// JSON marshals value and writes it as a JSON response. It marshals before
// writing the status, so a marshal error leaves the response untouched.
func (h Handle) JSON(code int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	h.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	h.w.WriteHeader(code)

	_, err = h.w.Write(append(data, '\n'))
	return err
}

// DecodeJSON decodes exactly one JSON value from the request body.
func (h Handle) DecodeJSON(v any) error {
	r := h.r
	if r.Body == nil {
		return errors.New("request body is empty")
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}

		return fmt.Errorf("decode trailing JSON data: %w", err)
	}

	return nil
}
