package environment

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Reader resolves process variables before dotenv values and records parse errors.
type Reader struct {
	values map[string]string
	err    error
}

func Load(filename string) (*Reader, error) {
	values, err := Read(filename)
	if err != nil {
		return nil, err
	}

	return &Reader{values: values}, nil
}

func (r *Reader) String(name, fallback string) string {
	if value, exists := os.LookupEnv(name); exists {
		return value
	}

	if value, exists := r.values[name]; exists {
		return value
	}

	return fallback
}

func (r *Reader) Int(name string, fallback int) int {
	value, err := strconv.Atoi(r.String(name, strconv.Itoa(fallback)))
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("environment: %s must be an integer", name)
	}

	return value
}

func (r *Reader) Int64(name string, fallback int64) int64 {
	value, err := strconv.ParseInt(r.String(name, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("environment: %s must be an integer", name)
	}

	return value
}

func (r *Reader) Duration(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(r.String(name, fallback.String()))
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("environment: %s must be a duration such as 1s or 5m", name)
	}

	return value
}

func (r *Reader) Bool(name string, fallback bool) bool {
	value, err := strconv.ParseBool(r.String(name, strconv.FormatBool(fallback)))
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("environment: %s must be a boolean", name)
	}

	return value
}

func (r *Reader) List(name string, fallback ...string) []string {
	text := r.String(name, strings.Join(fallback, ","))
	var values []string
	for _, item := range strings.Split(text, ",") {
		if value := strings.TrimSpace(item); value != "" {
			values = append(values, value)
		}
	}

	return values
}

func (r *Reader) Err() error {
	return r.err
}
