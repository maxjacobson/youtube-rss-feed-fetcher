# Run the program. Pass a channel URL: just run https://www.youtube.com/@eric-rosen
run *args:
    go run . {{args}}

# Run the tests
test:
    go test ./...

# Build the binary into bin/
build:
    go build -o bin/youtube-rss-feed-fetcher .

# Lint (go vet plus a gofmt check)
lint:
    go vet ./...
    test -z "$(gofmt -l .)"

# Install the binary into GOBIN
install:
    go install .
