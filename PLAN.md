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
