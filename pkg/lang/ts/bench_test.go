package ts

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParse runs the parser over the largest valid corpus file.
func BenchmarkParse(b *testing.B) {
	benchmarkValidate(b, "testdata/valid/*.ts", Validate)
}

// BenchmarkParseTSX runs the TSX parser over the largest valid corpus
// file.
func BenchmarkParseTSX(b *testing.B) {
	benchmarkValidate(b, "testdata/valid/*.tsx", ValidateTSX)
}

func benchmarkValidate(b *testing.B, pattern string, validate func(string) error) {
	files, err := filepath.Glob(pattern)
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
		if err := validate(string(source)); err != nil {
			b.Fatal(err)
		}
	}
}
