# Run the program. Pass a channel URL: just run https://www.youtube.com/@veritasium
run *args:
    go run . {{args}}

# Run the tests
test:
    go test ./...

# Build the binary
build:
    go build -o youtube-rss-feed-fetcher .

# Lint (go vet plus a gofmt check)
lint:
    go vet ./...
    test -z "$(gofmt -l .)"

# Install the binary into GOBIN
install:
    go install .
