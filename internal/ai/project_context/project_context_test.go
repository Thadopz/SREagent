package project_context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLoadWithConfigDisabled(t *testing.T) {
	got, err := LoadWithConfig(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty context when disabled, got %q", got)
	}
}

func TestLoadWithConfigMissingFileReturnsEmpty(t *testing.T) {
	got, err := LoadWithConfig(context.Background(), Config{
		Enabled:  true,
		Path:     filepath.Join(t.TempDir(), "missing.md"),
		MaxChars: 100,
	})
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty context for missing file, got %q", got)
	}
}

func TestLoadWithConfigReadsMarkdown(t *testing.T) {
	path := writeContext(t, "# Project\n\nUse evidence first.\n")
	got, err := LoadWithConfig(context.Background(), Config{
		Enabled:  true,
		Path:     path,
		MaxChars: 100,
	})
	if err != nil {
		t.Fatalf("expected context read, got err=%v", err)
	}
	if got != "# Project\n\nUse evidence first." {
		t.Fatalf("expected trimmed markdown, got %q", got)
	}
}

func TestLoadWithConfigTruncatesLongMarkdown(t *testing.T) {
	path := writeContext(t, strings.Repeat("a", 20))
	got, err := LoadWithConfig(context.Background(), Config{
		Enabled:  true,
		Path:     path,
		MaxChars: 7,
	})
	if err != nil {
		t.Fatalf("expected context read, got err=%v", err)
	}
	if utf8.RuneCountInString(got) != 7 {
		t.Fatalf("expected 7 runes, got %d", utf8.RuneCountInString(got))
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	got := LoadConfig(context.Background())
	if !got.Enabled {
		t.Fatal("expected project context enabled by default")
	}
	if got.Path != DefaultPath {
		t.Fatalf("expected default path %q, got %q", DefaultPath, got.Path)
	}
	if got.MaxChars != DefaultMaxChars {
		t.Fatalf("expected default max chars %d, got %d", DefaultMaxChars, got.MaxChars)
	}
}

func writeContext(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AGENT.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	return path
}
