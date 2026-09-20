package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// goldenDump builds expected dump output from the locked format rules
// (same layout as formatDump / writeDumpLine).
func goldenDump(ids []int, pieces []string, hexIDs bool) string {
	var b strings.Builder
	n := len(ids)
	if n == 0 {
		return "0000000\n"
	}
	for i := 0; i < n; i += idsPerRow {
		end := i + idsPerRow
		if end > n {
			end = n
		}
		rowIDs := ids[i:end]
		rowPieces := pieces[i:end]

		var line strings.Builder
		fmt.Fprintf(&line, "%07x  ", i)
		for _, id := range rowIDs {
			if hexIDs {
				fmt.Fprintf(&line, "%*x", idFieldWidth, id)
			} else {
				fmt.Fprintf(&line, "%*d", idFieldWidth, id)
			}
		}
		remaining := (idsPerRow - len(rowIDs)) * idFieldWidth
		line.WriteString(strings.Repeat(" ", remaining))
		for line.Len() < textCol-1 {
			line.WriteByte(' ')
		}
		line.WriteString(formatText(rowPieces))
		line.WriteByte('\n')
		b.WriteString(line.String())
	}
	fmt.Fprintf(&b, "%07x\n", n)
	return b.String()
}

func TestFormatHelloWorld(t *testing.T) {
	ids, pieces, err := tokenize("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := formatDump(&out, ids, pieces, false); err != nil {
		t.Fatal(err)
	}
	want := goldenDump(ids, pieces, false)
	if out.String() != want {
		t.Fatalf("format mismatch\ngot:\n%q\nwant:\n%q\n--- got ---\n%s--- want ---\n%s",
			out.String(), want, out.String(), want)
	}
	// Exact known golden for o200k "Hello world"
	exact := "0000000" + "  " +
		fmt.Sprintf("%7d", 13225) + fmt.Sprintf("%7d", 2375) +
		strings.Repeat(" ", 2*idFieldWidth)
	for len(exact) < textCol-1 {
		exact += " "
	}
	exact += "Hello|·world\n0000002\n"
	if out.String() != exact {
		t.Fatalf("exact golden mismatch\ngot:\n%q\nwant:\n%q\nvisible got:\n%s", out.String(), exact, out.String())
	}
}

func TestFormatEmpty(t *testing.T) {
	var out bytes.Buffer
	if err := formatDump(&out, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0000000\n" {
		t.Fatalf("empty = %q, want %q", out.String(), "0000000\n")
	}
}

func TestFormatNewline(t *testing.T) {
	ids, pieces, err := tokenize("a\nb")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := formatDump(&out, ids, pieces, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "↵") {
		t.Fatalf("expected ↵ in text column for newline; got:\n%s pieces=%#v", out.String(), pieces)
	}
	var stdout, stderr bytes.Buffer
	code := run(nil, strings.NewReader("a\nb"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "↵") {
		t.Fatalf("stdin path missing ↵: %q", stdout.String())
	}
}

func TestRunStdinHelloWorld(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, strings.NewReader("Hello world"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	ids, pieces, err := tokenize("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	want := goldenDump(ids, pieces, false)
	if out.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		var out, errBuf bytes.Buffer
		code := run([]string{arg}, strings.NewReader(""), &out, &errBuf)
		if code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, errBuf.String())
		}
		if !strings.Contains(out.String(), "Usage: tokdump") {
			t.Fatalf("%s: stdout missing usage: %q", arg, out.String())
		}
		if !strings.Contains(out.String(), "-v") {
			t.Fatalf("%s: stdout should mention -v: %q", arg, out.String())
		}
		if errBuf.Len() != 0 {
			t.Fatalf("%s: unexpected stderr %q", arg, errBuf.String())
		}
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"-v", "--version"} {
		var out, errBuf bytes.Buffer
		code := run([]string{arg}, strings.NewReader(""), &out, &errBuf)
		if code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, errBuf.String())
		}
		got := out.String()
		if !strings.HasPrefix(got, "tokdump ") {
			t.Fatalf("%s: stdout = %q, want prefix %q", arg, got, "tokdump ")
		}
		if !strings.Contains(got, version) {
			t.Fatalf("%s: stdout = %q, want to contain version %q", arg, got, version)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Fatalf("%s: stdout missing trailing newline: %q", arg, got)
		}
		if errBuf.Len() != 0 {
			t.Fatalf("%s: unexpected stderr %q", arg, errBuf.String())
		}
	}
}

func TestRunUnknownOption(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"-e"}, strings.NewReader(""), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown option")
	}
	if !strings.Contains(errBuf.String(), "unknown option") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "tokdump -h") {
		t.Fatalf("stderr should hint -h: %q", errBuf.String())
	}
}

func TestRunDoubleDashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "-h")
	if err := os.WriteFile(path, []byte("Hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"--", path}, strings.NewReader(""), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	ids, pieces, err := tokenize("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	want := goldenDump(ids, pieces, false)
	if out.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestRunMultiFileHeaders(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("Hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("Hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{a, b}, strings.NewReader(""), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "tokdump: "+a+":\n") {
		t.Fatalf("missing header for a: %q", s)
	}
	if !strings.Contains(s, "tokdump: "+b+":\n") {
		t.Fatalf("missing header for b: %q", s)
	}
}

func TestRunUnreadableContinues(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.txt")
	missing := filepath.Join(dir, "missing.txt")
	if err := os.WriteFile(ok, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{missing, ok}, strings.NewReader(""), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(errBuf.String(), missing) {
		t.Fatalf("stderr should mention missing file: %q", errBuf.String())
	}
	if !strings.Contains(out.String(), "tokdump: "+ok+":\n") {
		t.Fatalf("stdout should have header for ok file: %q", out.String())
	}
}

func TestDisplayTokenSubstitutions(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{" ", "·"},
		{"\n", "↵"},
		{"\r", `\r`},
		{"\t", `\t`},
		{"ab", "ab"},
		{"a b", "a·b"},
	}
	for _, c := range cases {
		if got := displayToken(c.in); got != c.want {
			t.Errorf("displayToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunEmptyStdin(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, strings.NewReader(""), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	if out.String() != "0000000\n" {
		t.Fatalf("empty stdin = %q", out.String())
	}
}

func TestFormatHelloWorldHex(t *testing.T) {
	ids, pieces, err := tokenize("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := formatDump(&out, ids, pieces, true); err != nil {
		t.Fatal(err)
	}
	want := goldenDump(ids, pieces, true)
	if out.String() != want {
		t.Fatalf("hex format mismatch\ngot:\n%q\nwant:\n%q\n%s", out.String(), want, out.String())
	}
	// 13225 = 0x33a9, 2375 = 0x947
	exact := "0000000" + "  " +
		fmt.Sprintf("%7x", 13225) + fmt.Sprintf("%7x", 2375) +
		strings.Repeat(" ", 2*idFieldWidth)
	for len(exact) < textCol-1 {
		exact += " "
	}
	exact += "Hello|·world\n0000002\n"
	if out.String() != exact {
		t.Fatalf("exact hex golden mismatch\ngot:\n%q\nwant:\n%q\n%s", out.String(), exact, out.String())
	}
}

func TestRunHexFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"-x"}, strings.NewReader("Hello world"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "33a9") {
		t.Fatalf("expected hex id 33a9 in output: %q", out.String())
	}
	if strings.Contains(out.String(), "13225") {
		t.Fatalf("decimal id should not appear with -x: %q", out.String())
	}
}

// The text column exists to make invisible and split characters legible.
// These are exactly the cases the old "." rendering destroyed.
func TestDisplayTokenEscapes(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"zero-width-space", "a\u200bb", "a\\u200bb"},
		{"zero-width-joiner", "a\u200db", "a\\u200db"},
		{"zero-width-non-joiner", "a\u200cb", "a\\u200cb"},
		{"bidi-rlo", "a\u202eb", "a\\u202eb"},
		{"bidi-lrm", "a\u200eb", "a\\u200eb"},
		{"nbsp", "a\u00a0b", "a\\u00a0b"},
		{"soft-hyphen", "a\u00adb", "a\\u00adb"},
		{"word-joiner", "a\u2060b", "a\\u2060b"},
		{"bom", "\ufeffa", "\\ufeffa"},
		{"noncharacter", "a\ufffeb", "a\\ufffeb"},
		{"private-use", "a\ue000b", "a\\ue000b"},
		{"astral-invisible", "a\U000e0041b", "a\\U000e0041b"},
		{"literal-pipe", "a|b", "a\\|b"},
		{"literal-backslash", "a\\b", "a\\\\b"},
		{"escape-lookalike", "\\u200b", "\\\\u200b"},
		{"tab", "a\tb", "a\\tb"},
		{"cr", "a\rb", "a\\rb"},
		{"nul", "a\x00b", "a\\x00b"},
		{"del", "a\x7fb", "a\\x7fb"},
		{"c1-control", "a\u0085b", "a\\u0085b"},
		{"space", "a b", "a\u00b7b"},
		{"newline", "a\nb", "a\u21b5b"},
		{"ascii", "hello", "hello"},
		{"cjk", "\u65e5\u672c\u8a9e", "\u65e5\u672c\u8a9e"},
		{"emoji-whole", "\U0001f44d", "\U0001f44d"},
		{"replacement-char", "a\ufffdb", "a\ufffdb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := displayToken(c.in); got != c.want {
				t.Errorf("displayToken(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// A token piece can begin or end mid-character, because one character may span
// several tokens. Those bytes are shown, not discarded.
func TestDisplayTokenSplitCharacter(t *testing.T) {
	// "\U0001f44d" is f0 9f 91 8d; a real split puts some bytes in one piece.
	if got := displayToken("\xf0\x9f"); got != "\\xf0\\x9f" {
		t.Errorf("leading half = %q, want %q", got, "\\xf0\\x9f")
	}
	if got := displayToken("\x91\x8d"); got != "\\x91\\x8d" {
		t.Errorf("trailing half = %q, want %q", got, "\\x91\\x8d")
	}
	// Valid text around an invalid fragment is still rendered literally.
	if got := displayToken("a\xf0\x9fb"); got != "a\\xf0\\x9fb" {
		t.Errorf("mixed = %q, want %q", got, "a\\xf0\\x9fb")
	}
}

// A combining mark at the start of a piece used to land on the "|" separator.
func TestDisplayTokenCombiningMarks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"mark-with-base", "e\u0301", "e\u0301"},
		{"mark-alone", "\u0301", "\u25cc\u0301"},
		{"stacked-marks-alone", "\u0315\u0306", "\u25cc\u0315\u0306"},
		{"stacked-marks-with-base", "a\u0315\u0306", "a\u0315\u0306"},
		{"mark-after-escape", "\u200b\u0301", "\\u200b\u25cc\u0301"},
		{"mark-after-space", " \u0301", "\u00b7\u25cc\u0301"},
		{"enclosing-mark-alone", "\u20dd", "\u25cc\u20dd"},
		{"devanagari-spacing-mark", "\u0915\u093e", "\u0915\u093e"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := displayToken(c.in); got != c.want {
				t.Errorf("displayToken(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Whatever a piece contains, the rendered column must stay on one line and be
// valid UTF-8, or the dump layout breaks.
func TestDisplayTokenOutputIsSafe(t *testing.T) {
	for _, in := range []string{
		"hello",
		"a\u200bb",
		"\u00f0\u009f",
		"\u0301",
		"a\nb",
		"a\tb",
		"\x00\x01\x02",
		"\U0001f468\u200d\U0001f469\u200d\U0001f467\u200d\U0001f466",
		"\u65e5\u672c\u8a9e",
		"a|b",
		"a\\b",
		"\ufeff",
		"\U0010ffff",
	} {
		got := displayToken(in)
		if !utf8.ValidString(got) {
			t.Errorf("displayToken(%q) produced invalid UTF-8", in)
		}
		if strings.ContainsAny(got, "\n\r\t") {
			t.Errorf("displayToken(%q) = %q leaked a raw control character", in, got)
		}
	}
}

// formatText must keep pieces separable: every "|" in the output is either a
// separator or an escaped literal, never an ambiguous mix.
func TestFormatTextSeparatorIsUnambiguous(t *testing.T) {
	got := formatText([]string{"a|b", "c", "|"})
	want := "a\\|b" + "|" + "c" + "|" + "\\|"
	if got != want {
		t.Fatalf("formatText = %q, want %q", got, want)
	}
	// Scanning past escapes must find exactly one separator between pieces.
	seps := 0
	for i := 0; i < len(got); i++ {
		if got[i] == '\\' {
			i++
			continue
		}
		if got[i] == '|' {
			seps++
		}
	}
	if seps != 2 {
		t.Fatalf("found %d unescaped separators, want 2 (in %q)", seps, got)
	}
}

// invalidUTF8Input is "ok " followed by a truncated 4-byte emoji.
var invalidUTF8Input = []byte("ok \xf0\x9f\x91")

func TestRunWarnsOnInvalidUTF8(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, bytes.NewReader(invalidUTF8Input), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (a warning must not fail the run)", code)
	}
	if !strings.Contains(out.String(), "0000000") {
		t.Fatalf("dump still expected on stdout: %q", out.String())
	}
	e := errBuf.String()
	if !strings.Contains(e, "warning") || !strings.Contains(e, "not valid UTF-8") {
		t.Fatalf("stderr = %q, want a UTF-8 warning", e)
	}
	if !strings.Contains(e, stdinName) {
		t.Fatalf("stderr should name stdin: %q", e)
	}
}

func TestRunNoWarningOnValidUTF8(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, strings.NewReader("hello 日本語 👍"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("unexpected stderr for valid input: %q", errBuf.String())
	}
}

func TestRunStrictRejectsInvalidUTF8(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"--strict"}, bytes.NewReader(invalidUTF8Input), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit under --strict")
	}
	if out.Len() != 0 {
		t.Fatalf("--strict must not print a dump: %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "not valid UTF-8") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
	if strings.Contains(errBuf.String(), "warning") {
		t.Fatalf("--strict should report an error, not a warning: %q", errBuf.String())
	}
}

func TestRunStrictAllowsValidUTF8(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"--strict"}, strings.NewReader("Hello world"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Hello|·world") {
		t.Fatalf("stdout = %q", out.String())
	}
}

// Under --strict a bad file is skipped, but the other files still dump.
func TestRunStrictSkipsBadFileContinues(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(good, []byte("Hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, invalidUTF8Input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"--strict", bad, good}, strings.NewReader(""), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if strings.Contains(out.String(), bad) {
		t.Fatalf("rejected file should not be dumped or headed: %q", out.String())
	}
	if !strings.Contains(out.String(), "tokdump: "+good+":\n") {
		t.Fatalf("good file should still be dumped: %q", out.String())
	}
}

func TestParseArgsStrict(t *testing.T) {
	opts, err := parseArgs([]string{"--strict", "-x", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.strict {
		t.Error("--strict not set")
	}
	if !opts.hex {
		t.Error("-x not set")
	}
	if len(opts.files) != 1 || opts.files[0] != "a.txt" {
		t.Errorf("files = %v", opts.files)
	}
}

// The help text must document the escapes, or the column is unreadable.
func TestHelpDocumentsEscapes(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"-h"}, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{`\xNN`, `\uXXXX`, `\|`, "◌", "·", "↵", "--strict"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}
