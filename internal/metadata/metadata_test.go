package metadata

import (
	"testing"
	"testing/fstest"
)

func TestEmbeddedTreeHashDeterministic(t *testing.T) {
	fsys := fstest.MapFS{
		"root/b.txt": {Data: []byte("beta")},
		"root/a.txt": {Data: []byte("alpha")},
	}

	first, err := EmbeddedTreeHash(fsys, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash() error = %v", err)
	}
	second, err := EmbeddedTreeHash(fsys, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash() second error = %v", err)
	}

	if first != second {
		t.Fatalf("hash must be deterministic, first=%q second=%q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("hash length = %d, want 64", len(first))
	}
}

func TestEmbeddedTreeHashTracksContentAndPath(t *testing.T) {
	base := fstest.MapFS{
		"root/a.txt": {Data: []byte("alpha")},
	}
	changedContent := fstest.MapFS{
		"root/a.txt": {Data: []byte("beta")},
	}
	changedPath := fstest.MapFS{
		"root/b.txt": {Data: []byte("alpha")},
	}

	baseHash, err := EmbeddedTreeHash(base, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(base) error = %v", err)
	}
	changedContentHash, err := EmbeddedTreeHash(changedContent, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(changedContent) error = %v", err)
	}
	changedPathHash, err := EmbeddedTreeHash(changedPath, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(changedPath) error = %v", err)
	}

	if baseHash == changedContentHash {
		t.Fatal("hash should change when file content changes")
	}
	if baseHash == changedPathHash {
		t.Fatal("hash should change when file path changes")
	}
}

func TestSeedContextTemplatePath(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{
			name: "default locale",
			want: "templates/seed/context/background.md.tmpl",
		},
		{
			name:   "localized",
			locale: "en-US",
			want:   "templates/seed/context/background.en-US.md.tmpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SeedContextTemplatePath("background", tt.locale)
			if got != tt.want {
				t.Fatalf("SeedContextTemplatePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
