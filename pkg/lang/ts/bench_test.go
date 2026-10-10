package ts

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParse runs the parser over the largest valid corpus file.
func BenchmarkParse(b *testing.B) {
	files, err := filepath.Glob("testdata/valid/*.ts")
	if err != nil || len(files) == 0 {
		b.Fatal("no corpus files")
	}
	largest := files[0]
	var source []byte
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			b.Fatal(err)
		}
		if len(data) > len(source) {
			largest, source = file, data
		}
	}
	b.Logf("benchmarking %s (%d bytes)", largest, len(source))
	b.ResetTimer()
	for b.Loop() {
		if err := Validate(string(source)); err != nil {
			b.Fatal(err)
		}
	}
}
