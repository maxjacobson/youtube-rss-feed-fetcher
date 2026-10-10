package youtube

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"adds https", "www.youtube.com/@veritasium", "https://www.youtube.com/@veritasium"},
		{"keeps https", "https://www.youtube.com/@veritasium", "https://www.youtube.com/@veritasium"},
		{"keeps http", "http://www.youtube.com/@veritasium", "http://www.youtube.com/@veritasium"},
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
			"handle URL has no channel ID",
			"https://www.youtube.com/@veritasium",
			"",
			false,
		},
		{
			"user URL has no channel ID",
			"https://www.youtube.com/user/veritasium",
			"",
			false,
		},
		{
			"custom URL has no channel ID",
			"https://www.youtube.com/c/veritasium",
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
			"embedded channelId JSON",
			`{"channelId":"` + id + `","foo":"bar"}`,
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
		shorts    bool
		want      string
	}{
		{
			"with shorts uses channel feed",
			id,
			true,
			"https://www.youtube.com/feeds/videos.xml?channel_id=" + id,
		},
		{
			"without shorts swaps UC for UULF",
			id,
			false,
			"https://www.youtube.com/feeds/videos.xml?playlist_id=UULFHnyfMqiRRG1u-2wmhEwi_d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FeedURL(tt.channelID, tt.shorts); got != tt.want {
				t.Errorf("FeedURL(%q, %v) = %q, want %q", tt.channelID, tt.shorts, got, tt.want)
			}
		})
	}
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

		got, err := ChannelID(server.URL+"/@veritasium", server.Client())
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
	const id = "UCHnyfMqiRRG1u-2wmhEwi_d"

	startServer := func(body string, status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, body)
		}))
	}

	t.Run("accepts a real Atom feed", func(t *testing.T) {
		server := startServer(`<?xml version="1.0"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <title>Veritasium</title>
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
		if err := VerifyFeed(FeedURL(id, true), &http.Client{Timeout: 1}); err == nil {
			t.Error("VerifyFeed() should have errored")
		}
	})
}
