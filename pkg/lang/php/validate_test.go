package php

import (
	"os"
	"path/filepath"
	"testing"
)

func corpusFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".php" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

func TestValidateValidCorpus(t *testing.T) {
	t.Parallel()
	for _, file := range corpusFiles(t, "testdata/valid") {
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
	for _, file := range corpusFiles(t, "testdata/invalid") {
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
