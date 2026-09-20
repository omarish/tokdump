package main

import (
	"strings"
	"unicode/utf8"
)

// Input that is not valid UTF-8 still has to be counted somehow. The count that
// matters is the one you get billed for, and the API only ever sees text that a
// JSON encoder produced, so the reference behaviour is the WHATWG "maximal
// subpart" rule that Python's bytes.decode(errors="replace") implements: each
// maximal invalid subsequence collapses to exactly one U+FFFD.
//
// Handing the raw bytes straight to the tokenizer does not do this. A Go string
// happily holds invalid bytes, but as soon as anything walks it as runes -- as
// the tokenizer's regex split does -- each invalid byte becomes its own U+FFFD.
// A truncated 4-byte emoji then contributes three U+FFFD instead of one, and
// since BPE merges runs of U+FFFD, that moves the token count.

// decodeUTF8 converts raw input bytes to a string, replacing each maximal
// invalid subpart with a single U+FFFD. It reports whether the input was
// already valid UTF-8.
func decodeUTF8(b []byte) (s string, valid bool) {
	if utf8.Valid(b) {
		return string(b), true
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for i := 0; i < len(b); {
		n, ok := utf8Sequence(b[i:])
		if ok {
			sb.Write(b[i : i+n])
		} else {
			sb.WriteRune(utf8.RuneError)
		}
		i += n
	}
	return sb.String(), false
}

// utf8Sequence returns the length of the UTF-8 sequence leading b and whether
// it is well-formed. When it is not, the length returned is the maximal
// subpart: the longest prefix that could still have grown into a valid
// sequence, which is what collapses to a single U+FFFD. It is always >= 1.
func utf8Sequence(b []byte) (n int, ok bool) {
	switch b0 := b[0]; {
	case b0 < 0x80:
		return 1, true
	case b0 < 0xC2:
		return 1, false // continuation byte, or overlong lead C0/C1
	case b0 < 0xE0:
		return contSeq(b, 2, 0x80, 0xBF)
	case b0 == 0xE0:
		return contSeq(b, 3, 0xA0, 0xBF) // no overlong 3-byte forms
	case b0 < 0xED:
		return contSeq(b, 3, 0x80, 0xBF)
	case b0 == 0xED:
		return contSeq(b, 3, 0x80, 0x9F) // no UTF-16 surrogates
	case b0 < 0xF0:
		return contSeq(b, 3, 0x80, 0xBF)
	case b0 == 0xF0:
		return contSeq(b, 4, 0x90, 0xBF) // no overlong 4-byte forms
	case b0 < 0xF4:
		return contSeq(b, 4, 0x80, 0xBF)
	case b0 == 0xF4:
		return contSeq(b, 4, 0x80, 0x8F) // no codepoints past U+10FFFF
	default:
		return 1, false // F5..FF
	}
}

// contSeq checks a sequence of total length n. Its first continuation byte must
// fall in [lo,hi] (which encodes the range restrictions that rule out overlongs,
// surrogates and out-of-range codepoints); the rest must be plain continuation
// bytes. A truncated or rejected sequence reports the length consumed so far,
// which is exactly the maximal subpart.
func contSeq(b []byte, n int, lo, hi byte) (int, bool) {
	for i := 1; i < n; i++ {
		lo, hi := lo, hi
		if i > 1 {
			lo, hi = 0x80, 0xBF
		}
		if i >= len(b) || b[i] < lo || b[i] > hi {
			return i, false
		}
	}
	return n, true
}
