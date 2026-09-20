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
4. **Text column** — starts at column 52; decoded token pieces separated by `|`

### Text column

A token piece is a run of **bytes**, not necessarily a whole character — one
emoji routinely spans several tokens. Anything that isn't plainly readable is
escaped rather than hidden, because the characters worth dumping are exactly
the ones that render as nothing:

| Rendering | Meaning |
|---|---|
| `·` | space (U+00B7 middle dot) |
| `↵` | newline (U+21B5) |
| `\t` `\r` | tab, carriage return |
| `\\` `\|` | a literal backslash, a literal pipe |
| `\xNN` | a control byte, or a byte of a character split across this token boundary |
| `\uXXXX` | a codepoint that renders as nothing: zero-width space, joiner, bidi control, BOM, soft hyphen, non-break space, unassigned |
| `◌x` | a combining mark with no base character before it in the piece |

Pieces are separated by `|`, and a literal `|` inside a piece is escaped as
`\|`, so the separator is never ambiguous.

After all tokens, a trailing line prints the next offset alone (hexdump habit).
Empty input prints only `0000000`.

Example (`echo -n 'Hello world' | tokdump`):

```
0000000    13225   2375                            Hello|·world
0000002
```

(IDs are width-7 right-aligned; the text column starts at column 52, with token
pieces separated by `|`.)

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

Token IDs print in **decimal** by default. Pass `-x` / `--hex` for hexadecimal (hexdump-style).

## Usage

```bash
# stdin
echo -n "Hello world" | tokdump

# one file
tokdump README.md

# multiple files (headers before each dump)
tokdump a.txt b.txt

# hexadecimal token IDs
tokdump -x a.txt

# help / version
tokdump -h
tokdump -v
```

## Unicode

The text column exists to make invisible characters visible. A zero-width
space, a bidi override and a soft hyphen are three very different problems,
and all three are why a prompt is longer than you expected:

```
$ printf 'a\u200bb\u202ec\u00add' | tokdump
0000000       64   3310     65 152821              a|\u200b|b|\u202e
0000004       66 130867                            c|\u00add
0000006
```

A character split across token boundaries shows its bytes, so you can see
exactly where the tokenizer cut:

```
$ printf '\U0001F468\u200D\U0001F469' | tokdump
0000000    28823    101   2524  28823              \xf0\x9f\x91|\xa8|\u200d|\xf0\x9f\x91
0000004      102                                   \xa9
0000005
```

Input is expected to be UTF-8. Input that **isn't** is still dumped, using the
same substitution an API client applies on the way out — each maximal invalid
byte sequence becomes one U+FFFD, per the WHATWG rule that Python's
`bytes.decode(errors="replace")` implements — and `tokdump` warns on stderr:

```
$ printf 'ok \xf0\x9f\x91' | tokdump
tokdump: warning: (standard input): not valid UTF-8; dumped with U+FFFD substitution
0000000      525  28151                            ok|·�
0000002
```

Use `--strict` to reject such input instead (no dump, non-zero exit).

## Conformance

`tokdump` is tested against the reference Python
[`tiktoken`](https://github.com/openai/tiktoken) across a corpus of unicode
edge cases — emoji and ZWJ sequences, CJK, RTL scripts, combining marks,
zero-width and bidi controls, BOMs, overlong and surrogate encodings,
truncated sequences, Latin-1 and UTF-16 mistaken for UTF-8, and binary junk.

- [`testdata/corpus.json`](testdata/corpus.json) — the inputs
- [`testdata/expected.json`](testdata/expected.json) — token IDs,
  **generated from real `tiktoken`**, never edited by hand
- [`testdata/dump_golden.txt`](testdata/dump_golden.txt) — the rendered dump
  for every case, so text-column changes show up as a reviewable diff
- [`scripts/gen_expected.py`](scripts/gen_expected.py) — regenerates the golden

`go test` needs no Python: it asserts against the committed answers. CI runs
the generator with `--check` on every push, so if `tiktoken` ever changes, or
someone edits the golden file by hand, the build fails with a diff.

```bash
go test ./...                              # offline
go test -run TestDumpGolden -update        # after an intentional format change
pip install tiktoken
python scripts/gen_expected.py --check     # what CI does
```

### Known upstream limitation

Three corpus cases are skipped, and the skips are tripwires that fail if the
cases start passing. `tiktoken-go` ships a code-generated regexp2 engine for
the `o200k_base` split pattern, and that engine mishandles the `\s*[\r\n]+`
alternative: a **blank line containing whitespace** splits into two pieces
where the reference tokenizer produces one.

```
input        reference   here
"a\n \nb"            3      4
"a\n\t\nb"           3      4
```

It costs one extra token per whitespace-only blank line, so it compounds on
text that has many of them. Ordinary blank lines, CRLF line endings, trailing
spaces and markdown hard breaks are all unaffected — none of the 35 real files
in these two repos hits it.

The bug is in the generated engine, not the pattern: interpreting the identical
pattern with `regexp2` directly gives the correct split. It is present in every
`tiktoken-go` release through v0.8.1. `pkoukk/tiktoken-go` (with its offline
loader, so still no network calls) tokenizes all three cases correctly and is
the likely fix.

## Man page & completions

- Man page: [`man/tokdump.1`](man/tokdump.1)
- Completions: [`completions/`](completions/) (`tokdump.bash`, `tokdump.zsh`, `tokdump.fish`)

## License

MIT © 2026 Omar Bohsali


Text-column tokens are separated with `|` (e.g. `37|01|c9|f2`).
