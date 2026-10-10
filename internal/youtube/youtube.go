// Package youtube resolves a YouTube channel URL to an RSS feed URL.
package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// ChannelIDPattern matches the UC... channel IDs YouTube uses.
const ChannelIDPattern = `UC[A-Za-z0-9_-]{22}`

var (
	channelInURLPath = regexp.MustCompile(`/channel/(` + ChannelIDPattern + `)`)
	channelIDInHTML  = []*regexp.Regexp{
		regexp.MustCompile(`"channelId":"(` + ChannelIDPattern + `)"`),
		regexp.MustCompile(`itemprop="identifier" content="(` + ChannelIDPattern + `)"`),
		regexp.MustCompile(`<link rel="canonical" href="[^"]*/channel/(` + ChannelIDPattern + `)"`),
	}
)

// NormalizeURL adds an https:// scheme if the URL is missing one.
func NormalizeURL(raw string) string {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return "https://" + raw
	}
	return raw
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
// for @handle, /c/ and /user/ style URLs.
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}

	if id, ok := ChannelIDFromHTML(string(body)); ok {
		return id, nil
	}
	return "", fmt.Errorf("couldn't find a channel ID at %s", url)
}

// FeedURL builds the RSS feed URL for a channel. With shorts it's the plain
// channel feed; without, the UC prefix is swapped for UULF, which points at
// the channel's uploads playlist feed and skips Shorts.
func FeedURL(channelID string, shorts bool) string {
	if shorts {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
	}
	if strings.HasPrefix(channelID, "UC") {
		channelID = "UULF" + channelID[2:]
	}
	return "https://www.youtube.com/feeds/videos.xml?playlist_id=" + channelID
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
