# tokdump

Like hexdump, but for tokens.

[![CI](https://github.com/omarish/tokdump/actions/workflows/ci.yml/badge.svg)](https://github.com/omarish/tokdump/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## What it does

`tokdump` dumps token IDs for files or stdin using OpenAI's **`o200k_base`**
encoding (the encoding used by GPT-4o and related models), in a classic
three-column hexdump-C style layout.

v1 has one job: plain `tokdump` always means `o200k_base`.
Options: `-h` / `--help` for usage, `-v` / `--version` for the version string.

A future `-e` / `--encoding` flag is planned; encoding selection is already
isolated in one place (`encoding()` in `encoding.go`) so that flag can plug in
without rewriting the CLI.

## Output format

Each dump line:

1. **Offset** — token index as 7 lowercase hex digits (`0000000`, `0000004`, …)
2. **Two spaces**
3. **Token IDs** — exactly **4 per row** (last row may have fewer), each
   right-aligned in width 7
4. **Text column** — starts at column 52; decoded token pieces with:
   - space ` ` → `·` (U+00B7)
   - newline `\n` → `↵` (U+21B5)
   - other non-printable runes (and `\r` / `\t`) → `.`

After all tokens, a trailing line prints the next offset alone (hexdump habit).
Empty input prints only `0000000`.

Example (`echo -n 'Hello world' | tokdump`):

```
0000000    13225   2375                            Hello·world
0000002
```

(IDs are width-7 right-aligned with no extra separators; text starts at column 52.)

With multiple files, each dump is preceded by `tokdump: FILENAME:`.

## Install

### Prebuilt binaries

Download the latest release for your OS/arch from:

**[github.com/omarish/tokdump/releases/latest](https://github.com/omarish/tokdump/releases/latest)**

Each release includes:

- Raw binaries (`tokdump_<os>_<arch>` / `.exe`) for direct download
- Archives (`tokdump_<version>_<os>_<arch>.tar.gz` or `.zip`) for packaging
- A `SHA256SUMS` file covering every asset (verify with `sha256sum -c SHA256SUMS`)

| Platform | Binary | Archive |
|---|---|---|
| macOS Apple Silicon | [`tokdump_darwin_arm64`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_darwin_arm64) | [`tokdump_*_darwin_arm64.tar.gz`](https://github.com/omarish/tokdump/releases/latest) |
| macOS Intel | [`tokdump_darwin_amd64`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_darwin_amd64) | [`tokdump_*_darwin_amd64.tar.gz`](https://github.com/omarish/tokdump/releases/latest) |
| Linux x86_64 | [`tokdump_linux_amd64`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_linux_amd64) | [`tokdump_*_linux_amd64.tar.gz`](https://github.com/omarish/tokdump/releases/latest) |
| Linux arm64 | [`tokdump_linux_arm64`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_linux_arm64) | [`tokdump_*_linux_arm64.tar.gz`](https://github.com/omarish/tokdump/releases/latest) |
| Windows x86_64 | [`tokdump_windows_amd64.exe`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_windows_amd64.exe) | [`tokdump_*_windows_amd64.zip`](https://github.com/omarish/tokdump/releases/latest) |
| Windows arm64 | [`tokdump_windows_arm64.exe`](https://github.com/omarish/tokdump/releases/latest/download/tokdump_windows_arm64.exe) | [`tokdump_*_windows_arm64.zip`](https://github.com/omarish/tokdump/releases/latest) |

Example (macOS Apple Silicon):

```bash
curl -L -o tokdump https://github.com/omarish/tokdump/releases/latest/download/tokdump_darwin_arm64
chmod +x tokdump
sudo mv tokdump /usr/local/bin/tokdump
```

> Links resolve after the first tagged release (`v0.1.0`). Until then, build from source below.

### From source

Requires Go 1.21+.

```bash
go install github.com/omarish/tokdump@latest
```

Or:

```bash
git clone https://github.com/omarish/tokdump.git
cd tokdump
make build
sudo mv tokdump /usr/local/bin/
```

## Usage

```bash
# stdin
echo -n "Hello world" | tokdump

# one file
tokdump README.md

# multiple files (headers before each dump)
tokdump a.txt b.txt

# help / version
tokdump -h
tokdump -v
```

## Man page & completions

- Man page: [`man/tokdump.1`](man/tokdump.1)
- Completions: [`completions/`](completions/) (`tokdump.bash`, `tokdump.zsh`, `tokdump.fish`)

## License

MIT © 2026 Omar Bohsali
