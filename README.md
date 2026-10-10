# youtube-rss-feed-fetcher

A small CLI that turns a YouTube channel URL into its RSS feed URL, and
copies it to your clipboard. Built in Go with the Charm TUI libraries.

```
youtube-rss-feed-fetcher https://www.youtube.com/@jamelle-bouie
```

Run it without an argument and it will prompt you to paste a URL in.

It accepts any style of channel URL (`/channel/UC...`, `@handle`, `/c/`,
`/user/`), checks which feeds the channel actually publishes (everything,
videos without Shorts, Shorts only, live streams), asks which one you want,
verifies that the feed responds, and puts the URL on your clipboard.

> [!NOTE]
> This is a vibecoded, just for fun little project. I am sure it is very
> imperfect in many ways, but it's kind of fun to be able to conjure things
> like this into existence.

## Why

YouTube publishes an RSS feed for every channel, which is handy for
subscribing to channels from an RSS reader without using the YouTube
subscription features. But:

1. The feed URL isn't linked anywhere on the channel page, so you have to
   know the `UC...` channel ID to construct it.
2. The default feed includes Shorts, which can drown out the regular
   uploads. Swapping the `UC` prefix for `UULF` points at the channel's
   uploads playlist feed instead, which skips Shorts. `UUSH` gives just
   the Shorts and `UULV` just the live streams — but those two 404 for
   channels without any, so the tool probes for them and only offers the
   feeds that exist.

## Installation

```
just install
```

(Requires a Go toolchain and [just](https://github.com/casey/just).)

## Development

```
just test   # run the tests
just lint   # go vet + gofmt check
just build  # build the binary
just run    # run the program, e.g. just run https://www.youtube.com/@ericrosen
```
