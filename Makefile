VERSION ?= dev

.PHONY: build test clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o tokdump .

test:
	go test ./...

clean:
	rm -f tokdump
