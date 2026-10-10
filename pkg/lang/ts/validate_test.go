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
