# youtube-rss-feed-fetcher

A small CLI that turns a YouTube channel URL into its RSS feed URL, and
copies it to your clipboard. Built in Go with the Charm TUI libraries.

```
youtube-rss-feed-fetcher https://www.youtube.com/@jamelle-bouie
```

It accepts any style of channel URL (`/channel/UC...`, `@handle`, `/c/`,
`/user/`), asks whether you want the feed to include Shorts, verifies that
the feed actually responds, and puts the URL on your clipboard.

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
   uploads playlist feed instead, which skips Shorts.

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
just run    # run the program, e.g. just run https://www.youtube.com/@veritasium
```
