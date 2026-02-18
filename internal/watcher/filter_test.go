package watcher

import "testing"

func TestSmartFilter(t *testing.T) {
	f := NewSmartFilter(nil)

	tests := []struct {
		path     string
		filtered bool
	}{
		{".git/config", true},
		{"node_modules/foo/bar.js", true},
		{"vendor/lib/thing.go", true},
		{"__pycache__/mod.pyc", true},
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"build/output.js", true},
		{"dist/bundle.js", true},
		{".next/cache/foo", true},
		{"target/debug/bin", true},
		{".venv/lib/python/site.py", true},
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"foo.pyc", true},
		{"bar.o", true},
		{"Baz.class", true},
		{".main.go.swp", true},
		{".model.go.swo", true},
		{"main.go~", true},
		{"4913", true},
		// Should NOT be filtered:
		{"src/app.go", false},
		{"README.md", false},
		{"internal/types/types.go", false},
		{"package.json", false},
		{"Makefile", false},
	}

	for _, tt := range tests {
		if got := f.IsFiltered(tt.path); got != tt.filtered {
			t.Errorf("IsFiltered(%q) = %v, want %v", tt.path, got, tt.filtered)
		}
	}
}

func TestSmartFilterWithExtra(t *testing.T) {
	f := NewSmartFilter([]string{"*.log", "tmp/"})

	if !f.IsFiltered("debug.log") {
		t.Error("expected debug.log to be filtered")
	}
	if !f.IsFiltered("tmp/cache.dat") {
		t.Error("expected tmp/cache.dat to be filtered")
	}
	if f.IsFiltered("src/main.go") {
		t.Error("expected src/main.go to NOT be filtered")
	}
}
