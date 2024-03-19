VERSION = $(shell git describe)

build:
	CGO_ENABLED=0 go build -ldflags="-X main.lyrebirdVersion=$(VERSION)" ./cmd/lyrebird
