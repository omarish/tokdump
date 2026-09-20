VERSION ?= dev

# Demo output is 16:9 for social embeds; vhs renders the terminal tight and
# the demo target centres it as a card.
DEMO_VER ?= v0.3.0

.PHONY: build test clean demo

build:
	go build -ldflags "-X main.version=$(VERSION)" -o tokdump .

test:
	go test ./...

clean:
	rm -f tokdump

# Render the release demo (GIF + MP4) from demo/tokdump.tape.
# Requires vhs: brew install vhs
demo: build
	@PATH="$(CURDIR):$$PATH" vhs demo/tokdump.tape
	@# Viewport already fits the content, so no padding pass is needed.
	@ffmpeg -v error -sseof -0.5 -i demo/.scratch.mp4 -vframes 1 demo/tokdump-$(DEMO_VER).png -y
	@rm -f demo/.scratch.mp4
	@ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of default=nw=1 demo/tokdump-$(DEMO_VER).png