package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// version is set at link time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `Usage: tokdump [file ...]
       tokdump -h | --help
       tokdump -v | --version

Like hexdump, but for tokens. Dump token IDs using OpenAI o200k_base
(GPT-4o and related models) in a classic three-column hexdump-C style.

With no files, read standard input and dump tokens.
With one or more files, dump each file; with more than one file, print a
header line before each file's dump.

Options:
  -h, --help      show this help
  -v, --version   print version and exit

Examples:
  echo -n "Hello world" | tokdump
  tokdump README.md
  tokdump a.txt b.txt
  cat notes.txt | tokdump

Encoding is always o200k_base in v1. A future -e/--encoding flag may
select other encodings; for now there are no dump options.
`

// run implements the hexdump-like CLI. Separated from main for testing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files, help, showVersion, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "tokdump: %v\n", err)
		return 1
	}
	if help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if showVersion {
		fmt.Fprintf(stdout, "tokdump %s\n", version)
		return 0
	}

	if len(files) == 0 {
		if err := dumpReader(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "tokdump: %v\n", err)
			return 1
		}
		return 0
	}

	var failed bool
	multi := len(files) > 1
	for _, name := range files {
		if multi {
			fmt.Fprintf(stdout, "tokdump: %s:\n", name)
		}
		if err := dumpFile(name, stdout); err != nil {
			fmt.Fprintf(stderr, "tokdump: %s: %s\n", name, errString(err))
			failed = true
			continue
		}
	}

	if failed {
		return 1
	}
	return 0
}

// parseArgs splits CLI args into file paths.
// -h / --help requests usage. -v / --version requests version.
// -- ends option parsing.
// Any other dash-led token is an error (so typos are not treated as filenames).
func parseArgs(args []string) (files []string, help, showVersion bool, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(files, args[i+1:]...), false, false, nil
		}
		if a == "-h" || a == "--help" {
			return nil, true, false, nil
		}
		if a == "-v" || a == "--version" {
			return nil, false, true, nil
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			return nil, false, false, fmt.Errorf("unknown option %s\nTry 'tokdump -h' for help.", a)
		}
		files = append(files, a)
	}
	return files, false, false, nil
}

func errString(err error) string {
	if pe, ok := err.(*fs.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func dumpFile(name string, stdout io.Writer) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return dumpReader(f, stdout)
}

func dumpReader(r io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	ids, pieces, err := tokenize(string(data))
	if err != nil {
		return err
	}
	return formatDump(stdout, ids, pieces)
}

const (
	idsPerRow    = 4
	idFieldWidth = 7
	textCol      = 52 // 1-based column where the text column starts
)

// formatDump writes classic hexdump-C-style token dump lines.
func formatDump(w io.Writer, ids []int, pieces []string) error {
	n := len(ids)
	if n == 0 {
		_, err := fmt.Fprintf(w, "%07x\n", 0)
		return err
	}

	for i := 0; i < n; i += idsPerRow {
		end := i + idsPerRow
		if end > n {
			end = n
		}
		if err := writeDumpLine(w, i, ids[i:end], pieces[i:end]); err != nil {
			return err
		}
	}
	// Trailing line: next offset alone (hexdump habit).
	_, err := fmt.Fprintf(w, "%07x\n", n)
	return err
}

func writeDumpLine(w io.Writer, offset int, ids []int, pieces []string) error {
	var b strings.Builder
	b.Grow(textCol + 64)

	// Offset (7 hex digits) + two spaces.
	fmt.Fprintf(&b, "%07x  ", offset)

	// ID block: always reserve idsPerRow * idFieldWidth characters.
	for _, id := range ids {
		fmt.Fprintf(&b, "%*d", idFieldWidth, id)
	}
	remaining := (idsPerRow - len(ids)) * idFieldWidth
	if remaining > 0 {
		b.WriteString(strings.Repeat(" ", remaining))
	}

	// Pad so the text column starts at 1-based column textCol.
	for b.Len() < textCol-1 {
		b.WriteByte(' ')
	}

	b.WriteString(formatText(pieces))
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

// formatText concatenates token pieces with hexdump-style substitutions.
func formatText(pieces []string) string {
	var b strings.Builder
	for _, p := range pieces {
		b.WriteString(displayToken(p))
	}
	return b.String()
}

// displayToken renders a single token piece for the text column.
func displayToken(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte → '.'
			b.WriteByte('.')
			s = s[1:]
			continue
		}
		s = s[size:]
		switch r {
		case ' ':
			b.WriteRune('·') // U+00B7 MIDDLE DOT
		case '\n':
			b.WriteRune('↵') // U+21B5
		case '\r', '\t':
			b.WriteByte('.')
		default:
			if unicode.IsPrint(r) {
				b.WriteRune(r)
			} else {
				b.WriteByte('.')
			}
		}
	}
	return b.String()
}
