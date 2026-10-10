package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "input.php")
	if err := os.WriteFile(file, []byte("<?php echo 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, arg := range args {
		if arg == "$file" {
			args[i] = file
		}
	}
	var stdout, stderr strings.Builder
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestValidateOK(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := run(t, "validate", "$file")
	if code != 0 || stderr != "" || !strings.Contains(stdout, ": ok") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestValidateError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "broken.php")
	if err := os.WriteFile(file, []byte("<?php\n$a = ;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := Run([]string{"validate", file}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout.String(), "ParseError at 2:6:") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestValidateLexError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "broken.php")
	if err := os.WriteFile(file, []byte("<?php $a = 08;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := Run([]string{"validate", file}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout.String(), "LexError at 1:12:") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestTokens(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := run(t, "tokens", "$file")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"open_tag", "echo", "int", ";", "eof"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	code, stdout, _ := run(t, "-list")
	if code != 0 || !strings.Contains(stdout, "php") {
		t.Fatalf("code %d, stdout %q", code, stdout)
	}
}

func TestUnknownExtension(t *testing.T) {
	t.Parallel()
	code, _, stderr := run(t, "validate", "/tmp/x.txt")
	if code != 2 || !strings.Contains(stderr, "no frontend for extension .txt") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	code, _, stderr := run(t, "explode", "$file")
	if code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestNoFiles(t *testing.T) {
	t.Parallel()
	code, _, stderr := run(t, "validate")
	if code != 2 || !strings.Contains(stderr, "no input files") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}
