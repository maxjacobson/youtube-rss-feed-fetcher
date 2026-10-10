package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunHelp(t *testing.T) {
	if err := run([]string{"--help"}); err != nil {
		t.Errorf("run(--help) error = %v", err)
	}
	if err := run([]string{"-h"}); err != nil {
		t.Errorf("run(-h) error = %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOpts options
		wantHelp bool
		wantErr  bool
	}{
		{"no args", nil, options{}, false, false},
		{
			"url only",
			[]string{"https://www.youtube.com/@eric-rosen"},
			options{url: "https://www.youtube.com/@eric-rosen"},
			false,
			false,
		},
		{"write flag only", []string{"--write"}, options{write: true}, false, false},
		{
			"write flag with url",
			[]string{"--write", "https://www.youtube.com/@eric-rosen"},
			options{url: "https://www.youtube.com/@eric-rosen", write: true},
			false,
			false,
		},
		{"long help", []string{"--help"}, options{}, true, false},
		{"short help", []string{"-h"}, options{}, true, false},
		{"unknown flag", []string{"--frobnicate"}, options{}, false, true},
		{"too many positional", []string{"a", "b"}, options{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, help, err := parseArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("parseArgs() should have errored")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs() error = %v", err)
			}
			if opts != tt.wantOpts || help != tt.wantHelp {
				t.Errorf("parseArgs(%v) = %+v, %v; want %+v, %v", tt.args, opts, help, tt.wantOpts, tt.wantHelp)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		path string
		want string
	}{
		{"~/Downloads/feeds.opml", filepath.Join(home, "Downloads", "feeds.opml")},
		{"~", home},
		{"feeds.opml", "feeds.opml"},
		{"/tmp/feeds.opml", "/tmp/feeds.opml"},
	}
	for _, tt := range tests {
		if got := expandPath(tt.path); got != tt.want {
			t.Errorf("expandPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDefaultOPMLPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	downloads := filepath.Join(home, "Downloads")
	mkdirAll := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	t.Run("base default when Downloads is empty", func(t *testing.T) {
		if got := defaultOPMLPath(); got != "~/Downloads/feeds.opml" {
			t.Errorf("defaultOPMLPath() = %q, want ~/Downloads/feeds.opml", got)
		}
	})

	t.Run("increments past existing files", func(t *testing.T) {
		mkdirAll(t, filepath.Join(downloads, "feeds.opml"))
		if got := defaultOPMLPath(); got != "~/Downloads/feeds (1).opml" {
			t.Errorf("defaultOPMLPath() = %q, want ~/Downloads/feeds (1).opml", got)
		}

		mkdirAll(t, filepath.Join(downloads, "feeds (1).opml"))
		if got := defaultOPMLPath(); got != "~/Downloads/feeds (2).opml" {
			t.Errorf("defaultOPMLPath() = %q, want ~/Downloads/feeds (2).opml", got)
		}
	})
}
