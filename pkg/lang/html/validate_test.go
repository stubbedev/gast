package html

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func corpusFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".html" {
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

func TestValidateErrorSpans(t *testing.T) {
	t.Parallel()
	source := "<p>\n  <span>x\n"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected an error")
	}
	var spanned gast.Spanned
	if !errors.As(err, &spanned) {
		t.Fatalf("error is not spanned: %T", err)
	}
	want := strings.Index(source, "<span")
	if got := spanned.Span(); got.Start != want || got.End != want+len("<span") {
		t.Fatalf("span: got %v, want %d..%d", got, want, want+len("<span>"))
	}
}

func TestValidateFormatErrorCaret(t *testing.T) {
	t.Parallel()
	source := "<p>\n  <span>x\n"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected an error")
	}
	formatted := gast.FormatError(err, source)
	want := strings.Join([]string{
		"ParseError at 2:3: unclosed element <span>",
		"2 |   <span>x",
		"  |   ^",
	}, "\n")
	if !strings.Contains(formatted, want) {
		t.Fatalf("formatted error:\n%s\nwant:\n%s", formatted, want)
	}
}
