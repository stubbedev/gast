package html

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParse parses the largest valid corpus file.
func BenchmarkParse(b *testing.B) {
	entries, err := os.ReadDir("testdata/valid")
	if err != nil {
		b.Fatal(err)
	}
	var source []byte
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".html" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata/valid", entry.Name()))
		if err != nil {
			b.Fatal(err)
		}
		if len(data) > len(source) {
			source = data
		}
	}
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for b.Loop() {
		if err := Validate(string(source)); err != nil {
			b.Fatal(err)
		}
	}
}
