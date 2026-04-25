package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
)

func TestGlobalIndigenousSourcesSeedFile(t *testing.T) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "scripts", "global-indigenous-sources.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open global indigenous sources seed file: %v", err)
	}
	defer file.Close()

	sources, err := importer.ParseIndigenousSources(file)
	if err != nil {
		t.Fatalf("parse global indigenous sources seed file: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("expected global indigenous sources seed file to contain sources")
	}

	seen := make(map[string]struct{}, len(sources))
	for i, src := range sources {
		if errMsg := importer.ValidateIndigenousSource(src); errMsg != "" {
			t.Fatalf("source %d (%q) failed validation: %s", i+1, src.Name, errMsg)
		}
		if _, ok := seen[src.Name]; ok {
			t.Fatalf("duplicate source name %q", src.Name)
		}
		seen[src.Name] = struct{}{}
	}
}
