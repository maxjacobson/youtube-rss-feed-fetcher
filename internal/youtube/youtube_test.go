package youtube

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"adds https", "www.youtube.com/@eric-rosen", "https://www.youtube.com/@eric-rosen"},
		{"keeps https", "https://www.youtube.com/@eric-rosen", "https://www.youtube.com/@eric-rosen"},
		{"keeps http", "http://www.youtube.com/@eric-rosen", "http://www.youtube.com/@eric-rosen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeURL(tt.raw); got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestChannelIDFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
		ok   bool
	}{
		{
			"channel URL",
			"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2wmhEwi_d",
			"UCHnyfMqiRRG1u-2wmhEwi_d",
			true,
		},
		{
			"channel URL with trailing slash",
			"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2wmhEwi_d/",
			"UCHnyfMqiRRG1u-2wmhEwi_d",
			true,
		},
		{
			"channel URL with query params",
			"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2wmhEwi_d?si=abc",
			"UCHnyfMqiRRG1u-2wmhEwi_d",
			true,
		},
		{
			"too-long ID doesn't get truncated",
			"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2wmhEwi_dX",
			"",
			false,
		},
		{
			"handle URL has no channel ID",
			"https://www.youtube.com/@eric-rosen",
			"",
			false,
		},
		{
			"user URL has no channel ID",
			"https://www.youtube.com/user/eric-rosen",
			"",
			false,
		},
		{
			"custom URL has no channel ID",
			"https://www.youtube.com/c/eric-rosen",
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ChannelIDFromURL(tt.url)
			if ok != tt.ok {
				t.Errorf("ChannelIDFromURL(%q) ok = %v, want %v", tt.url, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ChannelIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestChannelIDFromHTML(t *testing.T) {
	const id = "UCHnyfMqiRRG1u-2wmhEwi_d"
	tests := []struct {
		name string
		html string
		want string
		ok   bool
	}{
		{
			"embedded externalId JSON",
			`{"externalId":"` + id + `","foo":"bar"}`,
			id,
			true,
		},
		{
			"itemprop identifier",
			`<meta itemprop="identifier" content="` + id + `">`,
			id,
			true,
		},
		{
			"canonical link",
			`<link rel="canonical" href="https://www.youtube.com/channel/` + id + `">`,
			id,
			true,
		},
		{
			"embedded channelId JSON",
			`{"channelId":"` + id + `","foo":"bar"}`,
			id,
			true,
		},
		{
			"prefers page metadata over a recommended channel's channelId",
			`{"channelId":"UC_x68cJMKc3Bjy9LFISh3Hg"}` +
				`{"externalId":"` + id + `"}`,
			id,
			true,
		},
		{
			"nothing to find",
			`<html><body>hello</body></html>`,
			"",
			false,
		},
		{
			"embedded id is too short to count",
			`{"channelId":"UCshort"}`,
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ChannelIDFromHTML(tt.html)
			if ok != tt.ok {
				t.Errorf("ChannelIDFromHTML ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ChannelIDFromHTML = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFeedURL(t *testing.T) {
	const id = "UCHnyfMqiRRG1u-2wmhEwi_d"
	tests := []struct {
		name      string
		channelID string
		kind      Kind
		want      string
	}{
		{
			"everything uses the channel feed",
			id,
			KindEverything,
			"https://www.youtube.com/feeds/videos.xml?channel_id=" + id,
		},
		{
			"videos swaps UC for UULF",
			id,
			KindVideos,
			"https://www.youtube.com/feeds/videos.xml?playlist_id=UULFHnyfMqiRRG1u-2wmhEwi_d",
		},
		{
			"shorts swaps UC for UUSH",
			id,
			KindShorts,
			"https://www.youtube.com/feeds/videos.xml?playlist_id=UUSHHnyfMqiRRG1u-2wmhEwi_d",
		},
		{
			"live swaps UC for UULV",
			id,
			KindLive,
			"https://www.youtube.com/feeds/videos.xml?playlist_id=UULVHnyfMqiRRG1u-2wmhEwi_d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FeedURL(tt.channelID, tt.kind); got != tt.want {
				t.Errorf("FeedURL(%q, %v) = %q, want %q", tt.channelID, tt.kind, got, tt.want)
			}
		})
	}
}

func TestAvailableFeeds(t *testing.T) {
	const id = "UCHnyfMqiRRG1u-2wmhEwi_d"

	newServer := func(statusByPrefix map[string]int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			playlistID := r.URL.Query().Get("playlist_id")
			if len(playlistID) < 4 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if statusByPrefix[playlistID[:4]] != http.StatusOK {
				w.WriteHeader(statusByPrefix[playlistID[:4]])
				return
			}
			fmt.Fprint(w, `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
 <title>Videos</title>
 <author><name>Eric Rosen</name></author>
</feed>`)
		}))
	}

	t.Run("hides feeds the channel doesn't publish", func(t *testing.T) {
		server := newServer(map[string]int{
			"UULF": http.StatusOK,
			"UUSH": http.StatusNotFound,
			"UULV": http.StatusOK,
		})
		defer server.Close()

		got, title, err := AvailableFeeds(id, &http.Client{Transport: &redirectTransport{server: server}})
		if err != nil {
			t.Fatalf("AvailableFeeds() error = %v", err)
		}
		want := []Kind{KindEverything, KindVideos, KindLive}
		if !slices.Equal(got, want) {
			t.Errorf("AvailableFeeds() = %v, want %v", got, want)
		}
		if title != "Eric Rosen" {
			t.Errorf("AvailableFeeds() title = %q, want %q", title, "Eric Rosen")
		}
	})

	t.Run("offers only the channel feed when no tab feeds exist", func(t *testing.T) {
		server := newServer(map[string]int{
			"UULF": http.StatusNotFound,
			"UUSH": http.StatusNotFound,
			"UULV": http.StatusNotFound,
		})
		defer server.Close()

		got, title, err := AvailableFeeds(id, &http.Client{Transport: &redirectTransport{server: server}})
		if err != nil {
			t.Fatalf("AvailableFeeds() error = %v", err)
		}
		want := []Kind{KindEverything}
		if !slices.Equal(got, want) {
			t.Errorf("AvailableFeeds() = %v, want %v", got, want)
		}
		if title != "" {
			t.Errorf("AvailableFeeds() title = %q, want empty", title)
		}
	})

	t.Run("errors when a probe fails outright", func(t *testing.T) {
		server := newServer(map[string]int{
			"UULF": http.StatusTooManyRequests,
			"UUSH": http.StatusOK,
			"UULV": http.StatusOK,
		})
		defer server.Close()

		if _, _, err := AvailableFeeds(id, &http.Client{Transport: &redirectTransport{server: server}}); err == nil {
			t.Error("AvailableFeeds() should have errored")
		}
	})
}

// redirectTransport sends every request to a test server, keeping the
// original query string.
type redirectTransport struct {
	server *httptest.Server
}

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = strings.TrimPrefix(t.server.URL, "http://")
	return http.DefaultTransport.RoundTrip(cloned)
}

func TestChannelID(t *testing.T) {
	const id = "UCHnyfMqiRRG1u-2wmhEwi_d"

	t.Run("reads /channel/ URLs without fetching", func(t *testing.T) {
		client := &http.Client{}
		got, err := ChannelID("https://www.youtube.com/channel/"+id, client)
		if err != nil {
			t.Fatalf("ChannelID() error = %v", err)
		}
		if got != id {
			t.Errorf("ChannelID() = %q, want %q", got, id)
		}
	})

	t.Run("scrapes handle pages", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `<html><head><link rel="canonical" href="https://www.youtube.com/channel/%s"></head></html>`, id)
		}))
		defer server.Close()

		got, err := ChannelID(server.URL+"/@eric-rosen", server.Client())
		if err != nil {
			t.Fatalf("ChannelID() error = %v", err)
		}
		if got != id {
			t.Errorf("ChannelID() = %q, want %q", got, id)
		}
	})

	t.Run("errors when nothing is found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "<html><body>no channel here</body></html>")
		}))
		defer server.Close()

		if _, err := ChannelID(server.URL, server.Client()); err == nil {
			t.Error("ChannelID() should have errored")
		}
	})

	t.Run("reports an error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		_, err := ChannelID(server.URL, server.Client())
		if err == nil {
			t.Fatal("ChannelID() should have errored")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("ChannelID() error = %q, want it to mention the status", err)
		}
	})

	t.Run("errors on network failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		if _, err := ChannelID(url, server.Client()); err == nil {
			t.Error("ChannelID() should have errored")
		}
	})
}

func TestVerifyFeed(t *testing.T) {
	startServer := func(body string, status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, body)
		}))
	}

	t.Run("accepts a real Atom feed", func(t *testing.T) {
		server := startServer(`<?xml version="1.0"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <title>Eric Rosen</title>
</feed>`, http.StatusOK)
		defer server.Close()

		if err := VerifyFeed(server.URL, server.Client()); err != nil {
			t.Errorf("VerifyFeed() error = %v", err)
		}
	})

	t.Run("rejects an error status", func(t *testing.T) {
		server := startServer("not found", http.StatusNotFound)
		defer server.Close()

		if err := VerifyFeed(server.URL, server.Client()); err == nil {
			t.Error("VerifyFeed() should have errored")
		}
	})

	t.Run("rejects non-Atom XML", func(t *testing.T) {
		server := startServer(`<?xml version="1.0"?><html><body>nope</body></html>`, http.StatusOK)
		defer server.Close()

		if err := VerifyFeed(server.URL, server.Client()); err == nil {
			t.Error("VerifyFeed() should have errored")
		}
	})

	t.Run("rejects garbage", func(t *testing.T) {
		server := startServer("this is not xml at all", http.StatusOK)
		defer server.Close()

		if err := VerifyFeed(server.URL, server.Client()); err == nil {
			t.Error("VerifyFeed() should have errored")
		}
	})

	t.Run("errors when unreachable", func(t *testing.T) {
		server := startServer("", http.StatusOK)
		url := server.URL
		server.Close()

		if err := VerifyFeed(url, server.Client()); err == nil {
			t.Error("VerifyFeed() should have errored")
		}
	})
}
