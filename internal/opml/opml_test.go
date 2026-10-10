package opml

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func outlineTitles(doc string) []string {
	var parsed struct {
		Body struct {
			Outlines []struct {
				Text string `xml:"text,attr"`
			} `xml:"outline"`
		} `xml:"body"`
	}
	if err := xml.Unmarshal([]byte(doc), &parsed); err != nil {
		return nil
	}
	titles := make([]string, 0, len(parsed.Body.Outlines))
	for _, o := range parsed.Body.Outlines {
		titles = append(titles, o.Text)
	}
	return titles
}

func TestAddCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.opml")

	added, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !added {
		t.Error("Add() should have added a new outline")
	}

	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got := outlineTitles(string(doc)); len(got) != 1 || got[0] != "Eric Rosen (Videos)" {
		t.Errorf("outline titles = %v, want [Eric Rosen (Videos)]", got)
	}
	if !strings.Contains(string(doc), `xmlUrl="https://example.com/videos.xml"`) {
		t.Errorf("doc missing xmlUrl:\n%s", doc)
	}
	if !strings.Contains(string(doc), `type="rss"`) {
		t.Errorf("doc missing outline type:\n%s", doc)
	}
}

func TestAddAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.opml")

	if _, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	added, err := Add(path, "Jamelle Bouie (Live streams)", "https://example.com/live.xml")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !added {
		t.Error("Add() should have added a new outline")
	}

	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	want := []string{"Eric Rosen (Videos)", "Jamelle Bouie (Live streams)"}
	if got := outlineTitles(string(doc)); !slices.Equal(got, want) {
		t.Errorf("outline titles = %v, want %v", got, want)
	}
}

func TestAddDedupesByFeedURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.opml")

	if _, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	added, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added {
		t.Error("Add() should not have added a duplicate outline")
	}

	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got := outlineTitles(string(doc)); len(got) != 1 {
		t.Errorf("outline titles = %v, want one outline", got)
	}
}

func TestAddRejectsInvalidOPML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.opml")
	if err := os.WriteFile(path, []byte("this is not xml"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	if _, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml"); err == nil {
		t.Error("Add() should have errored on a non-OPML file")
	}
}

func TestAddLeavesExistingFileIntactWhenWriteFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feeds.opml")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if _, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml"); err == nil {
		t.Error("Add() should have errored when the directory isn't writable")
	}
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(doc) != "existing" {
		t.Errorf("file contents = %q, want %q", doc, "existing")
	}
}

func TestAddLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feeds.opml")

	if _, err := Add(path, "Eric Rosen (Videos)", "https://example.com/videos.xml"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "feeds.opml" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory entries = %v, want just feeds.opml", names)
	}
}
