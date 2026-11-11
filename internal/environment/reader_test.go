package environment

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReaderUsesProcessValuesBeforeFileAndDefaults(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(filename, []byte("COUNT=12\nTIMEOUT=2s\nENABLED=true\nLIST=one,two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("COUNT", "7")
	values, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}

	if values.Int("COUNT", 1) != 7 || values.Duration("TIMEOUT", time.Second) != 2*time.Second || !values.Bool("ENABLED", false) {
		t.Fatal("typed settings lost environment precedence")
	}

	if values.String("MISSING", "fallback") != "fallback" || values.Int64("MISSING_BYTES", 100) != 100 {
		t.Fatal("missing settings lost their defaults")
	}

	if list := values.List("LIST"); len(list) != 2 || list[1] != "two" {
		t.Fatalf("unexpected list: %v", list)
	}

	if err := values.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderRetainsParseErrors(t *testing.T) {
	values, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("INVALID_INT", "bad")
	values.Int("INVALID_INT", 1)
	first := values.Err()
	if first == nil {
		t.Fatal("invalid integer was accepted")
	}

	t.Setenv("INVALID_BOOL", "bad")
	values.Bool("INVALID_BOOL", false)
	if values.Err() != first {
		t.Fatal("a later parse error replaced the first error")
	}
}
