package main

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHuhThemeTitleColor(t *testing.T) {
	dark := huhTheme.Theme(true).Focused.Title.GetForeground()
	if dark != lipgloss.Color("51") {
		t.Errorf("dark title color = %v, want 51", dark)
	}
	light := huhTheme.Theme(false).Focused.Title.GetForeground()
	if light != lipgloss.Color("6") {
		t.Errorf("light title color = %v, want 6", light)
	}
}

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
