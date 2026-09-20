package main

import (
	"encoding/hex"
	"strings"
	"testing"
	"unicode/utf8"
)

// These cases are the ones where Go's own []byte -> string conversion disagrees
// with the reference decoder, plus the boundary conditions of the well-formed
// byte ranges. The full corpus in conformance_test.go covers the rest.
func TestDecodeUTF8(t *testing.T) {
	cases := []struct {
		name  string
		in    string // hex
		want  string // hex of the decoded UTF-8
		valid bool
	}{
		{"empty", "", "", true},
		{"ascii", "68656c6c6f", "68656c6c6f", true},
		{"two-byte", "c3a9", "c3a9", true},
		{"three-byte", "e697a5", "e697a5", true},
		{"four-byte", "f09f918d", "f09f918d", true},

		// One U+FFFD per *maximal subpart*, not per byte. This is the whole
		// reason the file exists: string(b) would give three here.
		{"truncated-4byte", "f09f91", "efbfbd", false},
		{"truncated-3byte", "e697", "efbfbd", false},
		{"truncated-2byte", "c3", "efbfbd", false},
		{"truncated-then-ascii", "f09f9161", "efbfbd61", false},

		// Each of these bytes is its own maximal subpart, so each yields one.
		{"lone-continuation", "80", "efbfbd", false},
		{"two-continuations", "8080", "efbfbdefbfbd", false},
		{"overlong-nul", "c080", "efbfbdefbfbd", false},
		{"overlong-3byte", "e08080", "efbfbdefbfbd" + "efbfbd", false},
		{"surrogate", "eda0bd", "efbfbdefbfbdefbfbd", false},
		{"f5-lead", "f5808080", "efbfbd" + "efbfbdefbfbdefbfbd", false},
		{"fe-ff", "feff", "efbfbdefbfbd", false},

		// Boundaries of the well-formed ranges.
		{"c2-min", "c280", "c280", true},
		{"c1-overlong-lead", "c181", "efbfbdefbfbd", false},
		{"e0-a0-min", "e0a080", "e0a080", true},
		{"e0-9f-overlong", "e09f80", "efbfbdefbfbdefbfbd", false},
		{"ed-9f-last-before-surrogates", "ed9fbf", "ed9fbf", true},
		{"f0-90-min", "f0908080", "f0908080", true},
		{"f0-8f-overlong", "f08f8080", "efbfbdefbfbdefbfbdefbfbd", false},
		{"f4-8f-max", "f48fbfbf", "f48fbfbf", true},
		{"f4-90-past-max", "f4908080", "efbfbdefbfbdefbfbdefbfbd", false},

		// Valid text with junk embedded keeps the valid parts byte-for-byte.
		{"mixed", "61e697a580f09f918d62", "61e697a5efbfbdf09f918d62", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, err := hex.DecodeString(c.in)
			if err != nil {
				t.Fatal(err)
			}
			got, valid := decodeUTF8(in)
			if gotHex := hex.EncodeToString([]byte(got)); gotHex != c.want {
				t.Errorf("decodeUTF8(%s) = %s, want %s", c.in, gotHex, c.want)
			}
			if valid != c.valid {
				t.Errorf("decodeUTF8(%s) valid = %v, want %v", c.in, valid, c.valid)
			}
			if !utf8.ValidString(got) {
				t.Errorf("decodeUTF8(%s) produced invalid UTF-8", c.in)
			}
		})
	}
}

// decodeUTF8 must never drop or duplicate valid text, and must always terminate
// with output that is itself valid UTF-8.
func TestDecodeUTF8Invariants(t *testing.T) {
	for _, s := range []string{
		"", "hello", "日本語", "👨‍👩‍👧‍👦", "café", "a​b", strings.Repeat("Ω", 100),
	} {
		got, valid := decodeUTF8([]byte(s))
		if !valid {
			t.Errorf("%q reported invalid", s)
		}
		if got != s {
			t.Errorf("valid input round-trip: got %q, want %q", got, s)
		}
	}
}

// The substitution tc must avoid does not happen at string(b) -- a Go string
// holds invalid bytes fine -- it happens when the tokenizer walks the input as
// runes, which yields one U+FFFD per invalid byte. This test pins the
// user-visible consequence: decoding first changes the count.
func TestDecodeUTF8ChangesTokenCount(t *testing.T) {
	raw := []byte("ok \xf0\x9f\x91") // "ok " + a truncated 4-byte emoji

	if n := len([]rune(string(raw))); n != 6 {
		t.Fatalf("rune decoding of raw input gave %d runes, want 6 (3 ASCII + 3 U+FFFD); Go behaviour changed?", n)
	}

	rawIDs, _, err := tokenize(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	decoded, valid := decodeUTF8(raw)
	if valid {
		t.Fatal("input should be reported invalid")
	}
	if n := strings.Count(decoded, string(utf8.RuneError)); n != 1 {
		t.Errorf("decoded U+FFFD count = %d, want 1", n)
	}
	decodedIDs, _, err := tokenize(decoded)
	if err != nil {
		t.Fatal(err)
	}

	// 2 is what the reference tiktoken reports; see testdata/expected.json,
	// case "invalid-truncated-4byte".
	if len(decodedIDs) != 2 {
		t.Errorf("decoded token count = %d, want 2", len(decodedIDs))
	}
	if len(rawIDs) == len(decodedIDs) {
		t.Errorf("tokenizing raw bytes gave the same answer (%d); this test no longer proves anything", len(rawIDs))
	}
}
