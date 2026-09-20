# Plan

Locked v1 decisions (implemented):

1. Dump tokens in files or stdin (hexdump-C style), not count.
2. Always OpenAI `o200k_base`.
3. CLI options: `-h` / `--help`, `-v` / `--version`; no encoding flags yet.
4. Encoding selection isolated in `encoding()` for a future `-e/--encoding`.
5. Module path `github.com/omarish/tokdump`.
6. Separate from `tc` (copy patterns, do not import tc).
7. Output format locked: 7-digit hex offset, 4 IDs/row width-7 right-aligned,
   text column at column 52, substitutions ` `→`·`, `\n`→`↵`, other non-print→`.`,
   trailing next-offset line.
8. Multi-file: print `tokdump: FILENAME:` header before each dump.
9. Unreadable file: error to stderr, continue, non-zero exit if any failed.

Ergonomics landed:

- Version via `main.version` + ldflags (`-v` / `--version`); Makefile + release workflow.
- Richer `-h` with Examples; man page `man/tokdump.1`.
- Shell completions (bash / zsh / fish).
- Release assets: raw binaries + `tokdump_<ver>_<os>_<arch>.tar.gz`/`.zip` + `SHA256SUMS`.

Unicode correctness (v0.2.0):

- Text column escapes instead of hiding. Invisible characters (zero-width,
  bidi, BOM, soft hyphen, non-break space) render as `\uXXXX`, bytes of a
  character split across a token boundary render as `\xNN`, and a combining
  mark with no base in the piece gets a dotted circle. Previously all of these
  collapsed to `.`, which made the tool useless for the case it exists for.
- `|` is escaped as `\|` inside a piece, so the piece separator is unambiguous.
- Input is decoded with the WHATWG maximal-subpart rule, so invalid UTF-8
  produces one U+FFFD per invalid *sequence*, matching Python's
  `bytes.decode(errors="replace")` and therefore the reference tiktoken.
- Invalid input warns on stderr before the dump; `--strict` rejects it instead.
- Conformance corpus in `testdata/`, generated from real tiktoken by
  `scripts/gen_expected.py`, plus a rendered-dump golden so text-column changes
  show up as a reviewable diff. `go test` runs offline; CI reruns the generator
  with `--check` to catch drift.
- Tokenizer is `pkoukk/tiktoken-go` with its offline loader. The previous
  library (`tiktoken-go/tokenizer`) ships a code-generated regexp2 engine that
  mis-splits `\s*[\r\n]+`, costing one extra token per whitespace-only blank
  line; the conformance corpus caught it and the swap fixed it. Binaries also
  got smaller (17M -> 15M) and still make no network calls.
