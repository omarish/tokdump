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

const usage = `Usage: tokdump [options] [file ...]
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
  -x, --hex       print token IDs in hexadecimal (default is decimal)

Examples:
  echo -n "Hello world" | tokdump
  echo -n "Hello world" | tokdump -x
  tokdump README.md
  tokdump -x a.txt b.txt

Encoding is always o200k_base in v1. A future -e/--encoding flag may
select other encodings.
`

type options struct {
	files   []string
	help    bool
	version bool
	hex     bool
}

// run implements the hexdump-like CLI. Separated from main for testing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "tokdump: %v\n", err)
		return 1
	}
	if opts.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if opts.version {
		fmt.Fprintf(stdout, "tokdump %s\n", version)
		return 0
	}

	if len(opts.files) == 0 {
		if err := dumpReader(stdin, stdout, opts.hex); err != nil {
			fmt.Fprintf(stderr, "tokdump: %v\n", err)
			return 1
		}
		return 0
	}

	var failed bool
	multi := len(opts.files) > 1
	for _, name := range opts.files {
		if multi {
			fmt.Fprintf(stdout, "tokdump: %s:\n", name)
		}
		if err := dumpFile(name, stdout, opts.hex); err != nil {
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

// parseArgs splits CLI args into options and file paths.
// -h / --help, -v / --version, -x / --hex.
// -- ends option parsing.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			return opts, nil
		}
		if a == "-h" || a == "--help" {
			opts.help = true
			return opts, nil
		}
		if a == "-v" || a == "--version" {
			opts.version = true
			return opts, nil
		}
		if a == "-x" || a == "--hex" {
			opts.hex = true
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			return options{}, fmt.Errorf("unknown option %s\nTry 'tokdump -h' for help.", a)
		}
		opts.files = append(opts.files, a)
	}
	return opts, nil
}

func errString(err error) string {
	if pe, ok := err.(*fs.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func dumpFile(name string, stdout io.Writer, hexIDs bool) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return dumpReader(f, stdout, hexIDs)
}

func dumpReader(r io.Reader, stdout io.Writer, hexIDs bool) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	ids, pieces, err := tokenize(string(data))
	if err != nil {
		return err
	}
	return formatDump(stdout, ids, pieces, hexIDs)
}

const (
	idsPerRow    = 4
	idFieldWidth = 7
	textCol      = 52 // 1-based column where the text column starts
)

// formatDump writes classic hexdump-C-style token dump lines.
func formatDump(w io.Writer, ids []int, pieces []string, hexIDs bool) error {
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
		if err := writeDumpLine(w, i, ids[i:end], pieces[i:end], hexIDs); err != nil {
			return err
		}
	}
	// Trailing line: next offset alone (hexdump habit).
	_, err := fmt.Fprintf(w, "%07x\n", n)
	return err
}

func writeDumpLine(w io.Writer, offset int, ids []int, pieces []string, hexIDs bool) error {
	var b strings.Builder
	b.Grow(textCol + 64)

	// Offset (7 hex digits) + two spaces.
	fmt.Fprintf(&b, "%07x  ", offset)

	// ID block: always reserve idsPerRow * idFieldWidth characters.
	for _, id := range ids {
		if hexIDs {
			fmt.Fprintf(&b, "%*x", idFieldWidth, id)
		} else {
			fmt.Fprintf(&b, "%*d", idFieldWidth, id)
		}
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

// formatText joins token pieces with "|" so variable-length tokens stay
// visually separable when they contain no spaces (e.g. hex hash fragments).
// Within each piece, space/newline/non-printables still use · / ↵ / .
func formatText(pieces []string) string {
	if len(pieces) == 0 {
		return ""
	}
	parts := make([]string, len(pieces))
	for i, p := range pieces {
		parts[i] = displayToken(p)
	}
	return strings.Join(parts, "|")
}

// displayToken renders a single token piece for the text column.
func displayToken(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
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
