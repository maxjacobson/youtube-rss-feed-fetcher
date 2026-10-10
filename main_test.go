package main

import "testing"

func TestRunHelp(t *testing.T) {
	if err := run([]string{"--help"}); err != nil {
		t.Errorf("run(--help) error = %v", err)
	}
	if err := run([]string{"-h"}); err != nil {
		t.Errorf("run(-h) error = %v", err)
	}
}
