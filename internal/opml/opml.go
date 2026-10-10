// Package opml writes OPML subscription files that feed readers can import.
package opml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const headTitle = "YouTube RSS feeds"

type outline struct {
	Type   string `xml:"type,attr"`
	Text   string `xml:"text,attr"`
	Title  string `xml:"title,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title string `xml:"title"`
	} `xml:"head"`
	Body struct {
		Outlines []outline `xml:"outline"`
	} `xml:"body"`
}

// Add appends an outline for the feed to the OPML file at path, creating
// the file if it doesn't exist. Outlines are deduped by feed URL, so adding
// the same feed twice is a no-op. It reports whether a new outline was added.
func Add(path, title, feedURL string) (bool, error) {
	var doc opmlDoc
	doc.Version = "2.0"
	doc.Head.Title = headTitle

	if data, err := os.ReadFile(path); err == nil {
		if err := xml.Unmarshal(data, &doc); err != nil {
			return false, fmt.Errorf("%s isn't a valid OPML file: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	for _, o := range doc.Body.Outlines {
		if o.XMLURL == feedURL {
			return false, nil
		}
	}

	doc.Body.Outlines = append(doc.Body.Outlines, outline{
		Type:   "rss",
		Text:   title,
		Title:  title,
		XMLURL: feedURL,
	})

	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	data = append([]byte(xml.Header), data...)
	data = append(data, '\n')
	return true, writeFile(path, data)
}

// writeFile replaces the file at path via a temp file and rename, so a
// crash mid-write can't destroy an existing file's contents.
func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
