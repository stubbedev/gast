package ts

import (
	"os"
	"path/filepath"
	"testing"
)

// corpusFiles collects corpus files with the given extensions from dir.
func corpusFiles(t *testing.T, dir string, exts ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		for _, ext := range exts {
			if filepath.Ext(entry.Name()) == ext {
				files = append(files, filepath.Join(dir, entry.Name()))
				break
			}
		}
	}
	return files
}

func TestValidateValidCorpus(t *testing.T) {
	t.Parallel()
	files := corpusFiles(t, "testdata/valid", ".ts")
	if len(files) < 14 {
		t.Fatalf("expected a full valid corpus, got %d files", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(string(source)); err != nil {
				t.Errorf("expected valid, got %v", err)
			}
		})
	}
}

func TestValidateInvalidCorpus(t *testing.T) {
	t.Parallel()
	files := corpusFiles(t, "testdata/invalid", ".ts")
	if len(files) < 12 {
		t.Fatalf("expected a full invalid corpus, got %d files", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(string(source)); err == nil {
				t.Error("expected a validation error, got none")
			}
		})
	}
}

func TestValidateValidTSXCorpus(t *testing.T) {
	t.Parallel()
	files := corpusFiles(t, "testdata/valid", ".tsx")
	if len(files) < 13 {
		t.Fatalf("expected a full valid TSX corpus, got %d files", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateTSX(string(source)); err != nil {
				t.Errorf("expected valid, got %v", err)
			}
		})
	}
}

func TestValidateInvalidTSXCorpus(t *testing.T) {
	t.Parallel()
	files := corpusFiles(t, "testdata/invalid", ".tsx")
	if len(files) < 12 {
		t.Fatalf("expected a full invalid TSX corpus, got %d files", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateTSX(string(source)); err == nil {
				t.Error("expected a validation error, got none")
			}
		})
	}
}

// TestValidateModeSeparation pins sources whose verdict flips with the
// mode, mirroring tsc with and without --jsx: JSX only in TSX mode,
// legacy angle assertions only in plain mode, and disambiguated
// generic arrows in both.
func TestValidateModeSeparation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		source string
		tsxOK  bool
		tsOK   bool
	}{
		{source: "const el = <div/>;", tsxOK: true, tsOK: false},
		{source: "const el = <>text</>;", tsxOK: true, tsOK: false},
		{source: "declare const x: unknown;\nconst y = <string>x;", tsxOK: false, tsOK: true},
		{source: "const f = <T>(v: T) => v;", tsxOK: false, tsOK: true},
		{source: "const f = <T,>(v: T) => v;", tsxOK: true, tsOK: true},
		{source: "const f = <T extends object>(v: T) => v;", tsxOK: true, tsOK: true},
		{source: "const cmp = 1 < 2 && 3 > 2;", tsxOK: true, tsOK: true},
		{source: "const cmp = a < b > (c);", tsxOK: true, tsOK: true},
	}
	for _, tc := range cases {
		if err := ValidateTSX(tc.source); (err == nil) != tc.tsxOK {
			t.Errorf("tsx %q: valid=%v, want %v (err %v)", tc.source, err == nil, tc.tsxOK, err)
		}
		if err := Validate(tc.source); (err == nil) != tc.tsOK {
			t.Errorf("ts %q: valid=%v, want %v (err %v)", tc.source, err == nil, tc.tsOK, err)
		}
	}
}
