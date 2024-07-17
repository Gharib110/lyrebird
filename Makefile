VERSION = $(shell git describe | sed 's/lyrebird-//')

build:
	CGO_ENABLED=0 go build -ldflags="-X main.lyrebirdVersion=$(VERSION)" ./cmd/lyrebird
