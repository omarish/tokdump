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
  -n, --narrow    2 token IDs per row instead of 4, for narrow terminals
      --strict    fail on input that is not valid UTF-8

Text column:
  Token pieces are separated by |. A piece is a run of bytes, not
  necessarily a whole character, so a character may span several tokens.
  Anything not plainly readable is escaped rather than hidden:

    ·  space          ↵  newline         \t \r  tab, carriage return
    \\ \|             literal backslash, literal pipe
    \xNN              a control byte, or a byte belonging to a character
                      that is split across this token boundary
    \uXXXX            a codepoint that would render as nothing: zero-width
                      space, joiner, bidi control, BOM, soft hyphen
    ◌x                a combining mark with no base character in the piece

Input must be UTF-8. Input that is not valid UTF-8 is tokenized the way an
API client would send it: each maximal invalid byte sequence becomes one
U+FFFD. tokdump warns on stderr when this happens. Use --strict to reject
such input instead.

Examples:
  echo -n "Hello world" | tokdump
  echo -n "Hello world" | tokdump -x
  printf 'a\u200bb' | tokdump      # reveals the zero-width space
  tokdump README.md
  tokdump -x a.txt b.txt

Encoding is always o200k_base in v1. A future -e/--encoding flag may
select other encodings.
`

// stdinName is how standard input is named in diagnostics.
const stdinName = "(standard input)"

type options struct {
	files   []string
	help    bool
	version bool
	hex     bool
	narrow  bool
	strict  bool
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
		text, valid, err := readInput(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "tokdump: %v\n", err)
			return 1
		}
		// Warn before dumping, so the diagnostic is not buried under the dump.
		if !valid {
			reportInvalid(stderr, stdinName, opts.strict)
			if opts.strict {
				return 1
			}
		}
		if err := dumpText(text, stdout, opts.hex, newLayout(opts.narrow)); err != nil {
			fmt.Fprintf(stderr, "tokdump: %v\n", err)
			return 1
		}
		return 0
	}

	var failed bool
	multi := len(opts.files) > 1
	for _, name := range opts.files {
		text, valid, err := readFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "tokdump: %s: %s\n", name, errString(err))
			failed = true
			continue
		}
		if !valid {
			reportInvalid(stderr, name, opts.strict)
			if opts.strict {
				failed = true
				continue
			}
		}
		if multi {
			fmt.Fprintf(stdout, "tokdump: %s:\n", name)
		}
		if err := dumpText(text, stdout, opts.hex, newLayout(opts.narrow)); err != nil {
			fmt.Fprintf(stderr, "tokdump: %s: %s\n", name, errString(err))
			failed = true
		}
	}

	if failed {
		return 1
	}
	return 0
}

// reportInvalid writes the diagnostic for input that is not valid UTF-8.
// Under --strict it is an error; otherwise it is a warning and the dump
// proceeds with U+FFFD substitution.
func reportInvalid(stderr io.Writer, name string, strict bool) {
	if strict {
		fmt.Fprintf(stderr, "tokdump: %s: not valid UTF-8\n", name)
		return
	}
	fmt.Fprintf(stderr, "tokdump: warning: %s: not valid UTF-8; dumped with U+FFFD substitution\n", name)
}

// parseArgs splits CLI args into options and file paths.
// -h / --help, -v / --version, -x / --hex, --strict.
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
		if a == "-n" || a == "--narrow" {
			opts.narrow = true
			continue
		}
		if a == "--strict" {
			opts.strict = true
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

func readFile(name string) (text string, valid bool, err error) {
	f, err := os.Open(name)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	return readInput(f)
}

// readInput reads r and decodes it, reporting whether it was valid UTF-8.
// Reading and dumping are separate so a warning can be emitted before the
// dump it applies to.
func readInput(r io.Reader) (text string, valid bool, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", false, err
	}
	text, valid = decodeUTF8(data)
	return text, valid, nil
}

// dumpText tokenizes text and writes the dump.
func dumpText(text string, stdout io.Writer, hexIDs bool, lay layout) error {
	ids, pieces, err := tokenize(text)
	if err != nil {
		return err
	}
	return formatDump(stdout, ids, pieces, hexIDs, lay)
}

const (
	offsetWidth  = 7 // hex digits in the offset field
	idFieldWidth = 7 // right-aligned width of one token ID
	gutter       = 2 // blank columns between the ID field and the text column
	pieceSep     = '|'
)

// layout is the column geometry of a dump. It follows hexdump(1): the offset,
// two spaces, a fully reserved ID field that is padded when a row is short, then
// the text column. hexdump also brackets its text column with "|", which tokdump
// cannot do because "|" already separates token pieces, so the bracket is
// dropped and the gutter is kept.
type layout struct {
	idsPerRow int
	textCol   int // 1-based column where the text column starts
}

// newLayout returns the wide (default) or narrow geometry. Narrow halves the
// IDs per row, which suits split panes and keeps long dumps from wrapping.
func newLayout(narrow bool) layout {
	n := 4
	if narrow {
		n = 2
	}
	return layout{
		idsPerRow: n,
		textCol:   offsetWidth + 2 + n*idFieldWidth + gutter + 1,
	}
}

// formatDump writes classic hexdump-C-style token dump lines.
func formatDump(w io.Writer, ids []int, pieces []string, hexIDs bool, lay layout) error {
	n := len(ids)
	if n == 0 {
		_, err := fmt.Fprintf(w, "%0*x\n", offsetWidth, 0)
		return err
	}

	for i := 0; i < n; i += lay.idsPerRow {
		end := i + lay.idsPerRow
		if end > n {
			end = n
		}
		if err := writeDumpLine(w, i, ids[i:end], pieces[i:end], hexIDs, lay); err != nil {
			return err
		}
	}
	// Trailing line: next offset alone (hexdump habit).
	_, err := fmt.Fprintf(w, "%0*x\n", offsetWidth, n)
	return err
}

func writeDumpLine(w io.Writer, offset int, ids []int, pieces []string, hexIDs bool, lay layout) error {
	var b strings.Builder
	b.Grow(lay.textCol + 64)

	// Offset (7 hex digits) + two spaces.
	fmt.Fprintf(&b, "%0*x  ", offsetWidth, offset)

	// ID block: always reserve idsPerRow * idFieldWidth characters.
	for _, id := range ids {
		if hexIDs {
			fmt.Fprintf(&b, "%*x", idFieldWidth, id)
		} else {
			fmt.Fprintf(&b, "%*d", idFieldWidth, id)
		}
	}
	remaining := (lay.idsPerRow - len(ids)) * idFieldWidth
	if remaining > 0 {
		b.WriteString(strings.Repeat(" ", remaining))
	}

	// Pad so the text column starts at 1-based column textCol.
	for b.Len() < lay.textCol-1 {
		b.WriteByte(' ')
	}

	b.WriteString(formatText(pieces))
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

// formatText joins token pieces with "|" so variable-length tokens stay
// visually separable when they contain no spaces (e.g. hex hash fragments).
// A literal "|" inside a piece is escaped, so the separator is unambiguous.
func formatText(pieces []string) string {
	if len(pieces) == 0 {
		return ""
	}
	parts := make([]string, len(pieces))
	for i, p := range pieces {
		parts[i] = displayToken(p)
	}
	return strings.Join(parts, string(pieceSep))
}

// displayToken renders a single token piece for the text column.
//
// A piece is a run of bytes, not necessarily a whole character: one
// astral-plane emoji routinely spans several tokens, so a piece can begin or
// end mid-character. Everything that is not plainly readable is escaped rather
// than collapsed to ".", because the characters worth dumping are exactly the
// ones that render as nothing.
//
//	\\ \|     literal backslash, literal pipe
//	· ↵       space, newline
//	\t \r     tab, carriage return
//	\xNN      a control byte, or a byte of a character split across this
//	          token boundary
//	\uXXXX    a codepoint that renders as nothing: zero-width space, joiner,
//	          bidi control, BOM, soft hyphen, non-break space, unassigned
//	◌x        a combining mark with no base character before it in the piece
func displayToken(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	// Tracks whether the last thing written was a literal character that a
	// combining mark can safely attach to. Without this, a piece that starts
	// with a combining mark puts the mark on the "|" separator.
	hasBase := false

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			// Not the start of a valid sequence. Expected here: it is how a
			// character split across token boundaries shows up.
			fmt.Fprintf(&b, `\x%02x`, s[i])
			hasBase = false
			i++
			continue
		}
		i += size

		if isCombining(r) {
			if !hasBase {
				b.WriteRune('◌') // U+25CC DOTTED CIRCLE
			}
			b.WriteRune(r)
			hasBase = true // further marks stack on this one
			continue
		}

		out := displayRune(r)
		b.WriteString(out)
		// Only an unescaped literal is a usable base for a following mark.
		hasBase = out == string(r)
	}
	return b.String()
}

// isCombining reports whether r is a mark that renders on top of the preceding
// character. Spacing marks (Mc) advance the cursor, so they stand on their own.
func isCombining(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
}

// displayRune renders one decoded codepoint.
func displayRune(r rune) string {
	switch r {
	case '\\':
		return `\\`
	case pieceSep:
		return `\|`
	case ' ':
		return "·" // U+00B7 MIDDLE DOT
	case '\n':
		return "↵" // U+21B5 DOWNWARDS ARROW WITH CORNER LEFTWARDS
	case '\t':
		return `\t`
	case '\r':
		return `\r`
	}
	switch {
	case r < 0x20 || r == 0x7f:
		// Remaining C0 controls and DEL, which arrive as single bytes.
		return fmt.Sprintf(`\x%02x`, r)
	case !unicode.IsPrint(r):
		// Zero-width and bidi controls, BOM, soft hyphen, non-ASCII spaces,
		// private use, unassigned. These are the ones worth naming: they are
		// invisible, and being invisible is why someone opened tokdump.
		return escapeRune(r)
	default:
		return string(r)
	}
}

func escapeRune(r rune) string {
	if r > 0xffff {
		return fmt.Sprintf(`\U%08x`, r)
	}
	return fmt.Sprintf(`\u%04x`, r)
}
