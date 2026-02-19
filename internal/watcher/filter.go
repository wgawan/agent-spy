package watcher

import (
	"path/filepath"
	"strings"
)

var defaultFilteredDirs = []string{
	".git",
	"node_modules",
	"vendor",
	".venv",
	"__pycache__",
	"build",
	"dist",
	".next",
	".nuxt",
	"target",
}

var defaultFilteredFiles = []string{
	".DS_Store",
	"Thumbs.db",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
}

var defaultFilteredExts = []string{
	".lock",
	".pyc",
	".o",
	".class",
	".swp",
	".swo",
	".swn",
}

type SmartFilter struct {
	filteredDirs  []string
	filteredFiles []string
	filteredExts  []string
	extraPatterns []string
}

func NewSmartFilter(extraPatterns []string) *SmartFilter {
	return &SmartFilter{
		filteredDirs:  defaultFilteredDirs,
		filteredFiles: defaultFilteredFiles,
		filteredExts:  defaultFilteredExts,
		extraPatterns: extraPatterns,
	}
}

func (f *SmartFilter) IsFiltered(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")

	// Check each path component against filtered directories
	for _, part := range parts {
		for _, dir := range f.filteredDirs {
			if part == dir {
				return true
			}
		}
	}

	// Check filename against filtered files
	base := filepath.Base(path)
	for _, name := range f.filteredFiles {
		if base == name {
			return true
		}
	}

	// Check extension
	ext := filepath.Ext(base)
	for _, filteredExt := range f.filteredExts {
		if ext == filteredExt {
			return true
		}
	}

	// Filter vim/editor temp files (~ suffix, 4913)
	if strings.HasSuffix(base, "~") || base == "4913" {
		return true
	}

	// Filter agent temp files (e.g. TECH_DOC.md.tmp.1482378.1771433725085)
	if strings.Contains(base, ".tmp.") {
		return true
	}

	// Check extra patterns
	for _, pattern := range f.extraPatterns {
		// Directory pattern (ends with /)
		if strings.HasSuffix(pattern, "/") {
			dirName := strings.TrimSuffix(pattern, "/")
			for _, part := range parts {
				if part == dirName {
					return true
				}
			}
			continue
		}
		// Glob pattern
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}
