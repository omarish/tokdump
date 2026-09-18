package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	exact += "Hello·world\n0000002\n"
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
		{"\r", "."},
		{"\t", "."},
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
	exact += "Hello·world\n0000002\n"
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
