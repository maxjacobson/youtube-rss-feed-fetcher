package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"

	"github.com/maxjacobson/youtube-rss-feed-fetcher/internal/opml"
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
  youtube-rss-feed-fetcher [flags] [channel URL]

Run without an argument to be prompted for the URL.
Accepts /channel/, @handle, /c/ and /user/ style URLs.

Flags:
  --write   also write the feed to an OPML file that feed readers can
            import. You're prompted for where to save it; the default is
            ~/Downloads/feeds.opml, incremented to feeds (1).opml etc. if
            a file by that name already exists.

Examples:
  youtube-rss-feed-fetcher https://www.youtube.com/@eric-rosen
  youtube-rss-feed-fetcher --write https://www.youtube.com/@eric-rosen
`

type options struct {
	url   string
	write bool
}

func parseArgs(args []string) (opts options, help bool, err error) {
	var positional []string
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			help = true
		case "--write":
			opts.write = true
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

	var url string
	if opts.url != "" {
		url = opts.url
	} else {
		var err error
		url, err = askForURL()
		if err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(1)
			}
			return err
		}
	}

	url = youtube.NormalizeURL(url)
	client := &http.Client{Timeout: 15 * time.Second}

	id, err := youtube.ChannelID(url, client)
	if err != nil {
		return err
	}

	kinds, channelTitle := youtube.AvailableFeeds(id, client)

	kind, err := pickKind(kinds, channelTitle)
	if err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(1)
		}
		return err
	}

	var opmlPath string
	if opts.write {
		opmlPath, err = askForPath(defaultOPMLPath())
		if err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(1)
			}
			return err
		}
	}

	feed := youtube.FeedURL(id, kind)

	if err := youtube.VerifyFeed(feed, client); err != nil {
		return err
	}

	if err := clipboard.WriteAll(feed); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", warnStyle.Render("warning: couldn't copy to clipboard"))
	}

	if opts.write {
		name := channelTitle
		if name == "" {
			name = id
		}
		outlineTitle := name + " (" + kind.ShortLabel() + ")"
		added, err := opml.Add(expandPath(opmlPath), outlineTitle, feed)
		if err != nil {
			return err
		}
		if added {
			fmt.Printf("%s %s\n", successIcon, titleStyle.Render("Added to "+opmlPath))
		} else {
			fmt.Printf("%s %s\n", successIcon, titleStyle.Render("Already in "+opmlPath))
		}
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
		Description("Paste any style of channel URL, e.g. https://www.youtube.com/@eric-rosen").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("please paste a channel URL")
			}
			return nil
		}).
		Value(&url).
		Run()
	return strings.TrimSpace(url), err
}

func askForPath(defaultPath string) (string, error) {
	path := defaultPath
	err := huh.NewInput().
		Title("Save OPML to").
		Description("A file feed readers can import; Enter accepts the default.").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("please enter a file path")
			}
			return nil
		}).
		Value(&path).
		Run()
	return strings.TrimSpace(path), err
}

func defaultOPMLPath() string {
	path := "~/Downloads/feeds.opml"
	if !pathExists(path) {
		return path
	}
	for i := 1; i < 100; i++ {
		path = fmt.Sprintf("~/Downloads/feeds (%d).opml", i)
		if !pathExists(path) {
			return path
		}
	}
	return path
}

func pathExists(path string) bool {
	_, err := os.Stat(expandPath(path))
	return err == nil
}

func expandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
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
		Run()
	return kind, err
}
