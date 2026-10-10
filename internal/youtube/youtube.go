// Package youtube resolves a YouTube channel URL to an RSS feed URL.
package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// ChannelIDPattern matches the UC... channel IDs YouTube uses.
const ChannelIDPattern = `UC[A-Za-z0-9_-]{22}`

var (
	// The ID must be followed by a non-ID character (or the end of the
	// string) so a longer ID-like segment isn't silently truncated to its
	// first 24 characters.
	channelInURLPath = regexp.MustCompile(`/channel/(` + ChannelIDPattern + `)(?:[^A-Za-z0-9_-]|$)`)
	channelIDInHTML  = []*regexp.Regexp{
		regexp.MustCompile(`"externalId":"(` + ChannelIDPattern + `)"`),
		regexp.MustCompile(`itemprop="identifier" content="(` + ChannelIDPattern + `)"`),
		regexp.MustCompile(`<link rel="canonical" href="[^"]*/channel/(` + ChannelIDPattern + `)"`),
		regexp.MustCompile(`"channelId":"(` + ChannelIDPattern + `)"`),
	}
)

// NormalizeURL trims surrounding whitespace and adds an https:// scheme
// if the URL is missing one. Scheme prefixes are matched case-insensitively
// so an HTTP:// URL is left alone rather than double-prefixed.
func NormalizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return trimmed
	}
	return "https://" + trimmed
}

// ChannelIDFromURL pulls a UC... channel ID out of a /channel/ style URL,
// like https://www.youtube.com/channel/UCxxxx.
func ChannelIDFromURL(url string) (string, bool) {
	m := channelInURLPath.FindStringSubmatch(url)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ChannelIDFromHTML scrapes a channel ID out of a rendered YouTube page,
// for @handle, /c/ and /user/ style URLs. Page-level metadata (externalId,
// itemprop, canonical link) is checked before channelId JSON blobs, which
// can name other channels that appear in the page's recommendations.
func ChannelIDFromHTML(html string) (string, bool) {
	for _, re := range channelIDInHTML {
		m := re.FindStringSubmatch(html)
		if m != nil {
			return m[1], true
		}
	}
	return "", false
}

// ChannelID resolves the channel ID for any style of channel URL. It reads
// it straight out of /channel/ URLs, and otherwise fetches the page and
// scrapes the ID out of the HTML.
func ChannelID(url string, client *http.Client) (string, error) {
	if id, ok := ChannelIDFromURL(url); ok {
		return id, nil
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}

	if id, ok := ChannelIDFromHTML(string(body)); ok {
		return id, nil
	}
	return "", fmt.Errorf("couldn't find a channel ID at %s", url)
}

// Kind identifies one of the RSS feeds a channel publishes. The Everything
// feed is the plain channel feed; the others are the hidden playlists
// behind the channel's Videos, Shorts and Live tabs, addressed by swapping
// the UC prefix of the channel ID.
type Kind int

const (
	KindEverything Kind = iota
	KindVideos
	KindShorts
	KindLive
)

// Label is the human-readable name of the feed, for pickers and menus.
func (k Kind) Label() string {
	switch k {
	case KindEverything:
		return "Everything (videos, Shorts, live)"
	case KindVideos:
		return "Videos only (no Shorts)"
	case KindShorts:
		return "Shorts only"
	case KindLive:
		return "Live streams only"
	}
	return "unknown"
}

// ShortLabel is a compact name for the feed, used in outline titles like
// "Eric Rosen (Videos)".
func (k Kind) ShortLabel() string {
	switch k {
	case KindEverything:
		return "Everything"
	case KindVideos:
		return "Videos"
	case KindShorts:
		return "Shorts"
	case KindLive:
		return "Live streams"
	}
	return "unknown"
}

var tabPlaylistPrefixes = map[Kind]string{
	KindVideos: "UULF",
	KindShorts: "UUSH",
	KindLive:   "UULV",
}

// FeedURL builds the RSS feed URL for a channel and kind of feed.
func FeedURL(channelID string, kind Kind) string {
	if prefix, ok := tabPlaylistPrefixes[kind]; ok && strings.HasPrefix(channelID, "UC") {
		return "https://www.youtube.com/feeds/videos.xml?playlist_id=" + prefix + channelID[2:]
	}
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
}

// AvailableFeeds returns the kinds of feed the channel actually publishes,
// in menu order, along with the channel's display name (from the feed's
// author element, empty if it couldn't be determined). The channel feed
// itself always exists for a valid channel; the tab playlist feeds 404 when
// the channel has no content for that tab, so they get probed. A probe that
// fails for any other reason (network error, unexpected status, malformed
// feed) is returned as an error rather than mistaken for an unpublished
// feed. The probes run concurrently.
func AvailableFeeds(channelID string, client *http.Client) ([]Kind, string, error) {
	probes := make([]feedProbe, len(probeKinds))
	errs := make([]error, len(probeKinds))
	var wg sync.WaitGroup
	for i, kind := range probeKinds {
		wg.Add(1)
		go func(i int, kind Kind) {
			defer wg.Done()
			probes[i], errs[i] = probeFeed(FeedURL(channelID, kind), client)
		}(i, kind)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, "", fmt.Errorf("couldn't check the %s feed: %w", probeKinds[i].ShortLabel(), err)
		}
	}

	available := []Kind{KindEverything}
	title := ""
	for i, probe := range probes {
		if probe.reachable {
			available = append(available, probeKinds[i])
			if title == "" {
				title = probe.title
			}
		}
	}
	return available, title, nil
}

var probeKinds = []Kind{KindVideos, KindShorts, KindLive}

type feedProbe struct {
	reachable bool
	title     string
}

// probeFeed checks whether one feed exists. A 404 means the channel doesn't
// publish it and isn't an error; anything else that goes wrong is.
func probeFeed(feedURL string, client *http.Client) (feedProbe, error) {
	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return feedProbe{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return feedProbe{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return feedProbe{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return feedProbe{}, fmt.Errorf("feed returned %s", resp.Status)
	}

	var feed struct {
		Author struct {
			Name string `xml:"name"`
		} `xml:"author"`
	}
	dec := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&feed); err != nil {
		return feedProbe{}, fmt.Errorf("%s isn't serving a valid feed: %w", feedURL, err)
	}
	return feedProbe{reachable: true, title: feed.Author.Name}, nil
}

// VerifyFeed checks that a feed URL responds and serves a real Atom feed.
func VerifyFeed(feedURL string, client *http.Client) error {
	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feed returned %s", resp.Status)
	}

	var feed struct {
		XMLName xml.Name
		Title   string `xml:"title"`
	}
	dec := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&feed); err != nil {
		return fmt.Errorf("%s isn't serving a valid feed: %w", feedURL, err)
	}
	if feed.XMLName.Local != "feed" {
		return fmt.Errorf("%s isn't serving an Atom feed", feedURL)
	}
	return nil
}
