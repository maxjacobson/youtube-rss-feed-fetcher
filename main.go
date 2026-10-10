package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"

	"github.com/maxjacobson/youtube-rss-feed-fetcher/internal/youtube"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	successIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render("✔")
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	feedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true)
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "youtube-rss-feed-fetcher: %v\n", err)
		os.Exit(1)
	}
}

const usage = `Print the RSS feed URL for a YouTube channel and copy it to your clipboard.

Usage:
  youtube-rss-feed-fetcher <channel URL>

Accepts /channel/, @handle, /c/ and /user/ style URLs.

Examples:
  youtube-rss-feed-fetcher https://www.youtube.com/@veritasium
  youtube-rss-feed-fetcher https://www.youtube.com/channel/UCxxxx
`

func run(args []string) error {
	if len(args) == 1 {
		switch args[0] {
		case "-h", "--help":
			fmt.Print(usage)
			return nil
		}
	}
	if len(args) != 1 {
		return fmt.Errorf("provide exactly one argument, a YouTube channel URL (try --help)")
	}

	url := youtube.NormalizeURL(args[0])
	client := &http.Client{Timeout: 15 * time.Second}

	id, err := youtube.ChannelID(url, client)
	if err != nil {
		return err
	}

	kinds := youtube.AvailableFeeds(id, client)

	kind, err := pickKind(kinds)
	if err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(1)
		}
		return err
	}

	feed := youtube.FeedURL(id, kind)

	if err := youtube.VerifyFeed(feed, client); err != nil {
		return err
	}

	if err := clipboard.WriteAll(feed); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", warnStyle.Render("warning: couldn't copy to clipboard"))
	}

	fmt.Printf("%s %s\n%s\n",
		successIcon,
		titleStyle.Render("Feed copied to clipboard"),
		feedStyle.Render(feed))
	return nil
}

func pickKind(kinds []youtube.Kind) (youtube.Kind, error) {
	kind := kinds[0]
	options := make([]huh.Option[youtube.Kind], 0, len(kinds))
	for _, k := range kinds {
		options = append(options, huh.NewOption(k.Label(), k))
	}
	err := huh.NewSelect[youtube.Kind]().
		Title("Which feed?").
		Description("Feeds this channel doesn't publish are hidden.").
		Options(options...).
		Value(&kind).
		Run()
	return kind, err
}
