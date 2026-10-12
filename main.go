package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// huhTheme is the Charm theme with its muted indigo field titles swapped
// for bright cyan, which keeps better contrast on more terminals.
var huhTheme huh.ThemeFunc = func(isDark bool) *huh.Styles {
	styles := huh.ThemeCharm(isDark)
	lightDark := lipgloss.LightDark(isDark)
	cyan := lightDark(lipgloss.Color("6"), lipgloss.Color("51"))
	styles.Focused.Title = styles.Focused.Title.Foreground(cyan)
	styles.Focused.NoteTitle = styles.Focused.NoteTitle.Foreground(cyan)
	styles.Blurred.Title = styles.Focused.Title
	styles.Blurred.NoteTitle = styles.Focused.NoteTitle
	styles.Group.Title = styles.Focused.Title
	return styles
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintf(os.Stderr, "youtube-rss-feed-fetcher: %v\n", err)
		}
		os.Exit(1)
	}
}

const usage = `Print the RSS feed URL for a YouTube channel and copy it to your clipboard.

Usage:
  youtube-rss-feed-fetcher [flags] [channel URL]

Run without an argument to be prompted for the URL. After each channel
it prompts for another; press Enter on an empty prompt to stop.
Accepts /channel/, @handle, /c/ and /user/ style URLs.

Examples:
  youtube-rss-feed-fetcher https://www.youtube.com/@eric-rosen
`

type options struct {
	url string
}

func parseArgs(args []string) (opts options, help bool, err error) {
	var positional []string
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			help = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, fmt.Errorf("unknown flag %s (try --help)", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return opts, false, fmt.Errorf("provide at most one argument, a YouTube channel URL (try --help)")
	}
	if len(positional) == 1 {
		opts.url = positional[0]
	}
	return opts, help, nil
}

func run(args []string) error {
	opts, help, err := parseArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Print(usage)
		return nil
	}

	client := &http.Client{Timeout: 15 * time.Second}

	if opts.url != "" {
		return processChannel(opts.url, client)
	}

	for {
		url, err := askForURL()
		if err != nil {
			return err
		}
		if url == "" {
			return nil
		}
		if err := processChannel(url, client); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s\n", warnStyle.Render("warning: "+err.Error()))
		}
		fmt.Println()
	}
}

func processChannel(url string, client *http.Client) error {
	url = youtube.NormalizeURL(url)

	id, err := youtube.ChannelID(url, client)
	if err != nil {
		return err
	}

	kinds, channelTitle, err := youtube.AvailableFeeds(id, client)
	if err != nil {
		return err
	}

	kind, err := pickKind(kinds, channelTitle)
	if err != nil {
		return err
	}

	feed := youtube.FeedURL(id, kind)

	if err := youtube.VerifyFeed(feed, client); err != nil {
		return err
	}

	if err := clipboard.WriteAll(feed); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", warnStyle.Render("warning: couldn't copy to clipboard"))
	}

	message := "Feed copied to clipboard"
	if channelTitle != "" {
		message = channelTitle + " feed copied to clipboard"
	}
	fmt.Printf("%s %s\n%s\n",
		successIcon,
		titleStyle.Render(message),
		feedStyle.Render(feed))
	return nil
}

func askForURL() (string, error) {
	url := ""
	err := huh.NewInput().
		Title("Channel URL").
		Description("Paste any style of channel URL, or press Enter to stop.").
		Value(&url).
		WithTheme(huhTheme).
		Run()
	return strings.TrimSpace(url), err
}

func pickKind(kinds []youtube.Kind, channelTitle string) (youtube.Kind, error) {
	kind := kinds[0]
	options := make([]huh.Option[youtube.Kind], 0, len(kinds))
	for _, k := range kinds {
		options = append(options, huh.NewOption(k.Label(), k))
	}
	title := "Which feed?"
	if channelTitle != "" {
		title = "Which feed for " + channelTitle + "?"
	}
	err := huh.NewSelect[youtube.Kind]().
		Title(title).
		Description("Feeds this channel doesn't publish are hidden.").
		Options(options...).
		Value(&kind).
		WithTheme(huhTheme).
		Run()
	return kind, err
}
