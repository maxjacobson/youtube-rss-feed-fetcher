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

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("provide exactly one argument, a YouTube channel URL")
	}

	url := youtube.NormalizeURL(args[0])
	client := &http.Client{Timeout: 15 * time.Second}

	id, err := youtube.ChannelID(url, client)
	if err != nil {
		return err
	}

	shorts, err := pickShorts()
	if err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(1)
		}
		return err
	}

	feed := youtube.FeedURL(id, shorts)

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

func pickShorts() (bool, error) {
	shorts := true
	err := huh.NewSelect[bool]().
		Title("Which feed?").
		Description("Channels publish two RSS feeds; the uploads-only one skips Shorts.").
		Options(
			huh.NewOption("Include Shorts", true),
			huh.NewOption("No Shorts", false),
		).
		Value(&shorts).
		Run()
	return shorts, err
}
