package environment

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadLiteralDotenvValues(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	contents := "# Settings\nexport REQUESTS = 20 # allowance\nINTERVAL='1s' # duration\nTEXT=\"a\\nb\"\nEMPTY=\nLITERAL=$(do-not-run)\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := Read(filename)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"REQUESTS": "20",
		"INTERVAL": "1s",
		"TEXT":     "a\nb",
		"EMPTY":    "",
		"LITERAL":  "$(do-not-run)",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %#v, want %#v", values, want)
	}
}

func TestReadRejectsMalformedAssignments(t *testing.T) {
	for _, contents := range []string{
		"MISSING_ASSIGNMENT",
		"1INVALID=value",
		"BAD-NAME=value",
		"KEY='unterminated",
		`KEY="bad\qescape"`,
		`KEY="valid" extra`,
	} {
		t.Run(contents, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := Read(filename); err == nil {
				t.Fatal("malformed assignment was accepted")
			}
		})
	}
}

func TestReadMissingFileIsOptional(t *testing.T) {
	values, err := Read(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(values) != 0 {
		t.Fatalf("got %v, %v", values, err)
	}
}
