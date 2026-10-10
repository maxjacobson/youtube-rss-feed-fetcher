// Package opml writes OPML subscription files that feed readers can import.
package opml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
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
	return true, os.WriteFile(path, data, 0o644)
}
