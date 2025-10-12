VERSION := $(shell git describe --tags --always)

build:
	go build -ldflags="-X 'github.com/goverture/goxy/config.Version=$(VERSION)'" -o goxy ./cmd/goxy

run:
	./goxy -v
