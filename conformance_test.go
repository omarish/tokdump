package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"
)

// The corpus and the expected answers live in testdata/. expected.json is
// generated from the reference Python tiktoken by scripts/gen_expected.py, and
// CI reruns that script with --check so the committed answers cannot silently
// drift. These tests therefore pin tokdump to real tiktoken without needing
// Python on the machine running `go test`.

var updateGolden = flag.Bool("update", false, "rewrite testdata/dump_golden.txt")

type corpusCase struct {
	Name string `json:"name"`
	Note string `json:"note"`
	Text string `json:"text"`
	Hex  string `json:"hex"`
}

type corpusFile struct {
	Encoding string       `json:"encoding"`
	Cases    []corpusCase `json:"cases"`
}

type expectedCase struct {
	ValidUTF8  bool   `json:"valid_utf8"`
	DecodedHex string `json:"decoded_hex"`
	Tokens     int    `json:"tokens"`
	IDs        []int  `json:"ids"`
}

type expectedFile struct {
	Encoding        string                  `json:"encoding"`
	TiktokenVersion string                  `json:"tiktoken_version"`
	Cases           map[string]expectedCase `json:"cases"`
}

// bytes returns the raw input for a case: readable cases carry text, byte-level
// cases carry hex.
func (c corpusCase) bytes(t *testing.T) []byte {
	t.Helper()
	if c.Hex != "" {
		b, err := hex.DecodeString(c.Hex)
		if err != nil {
			t.Fatalf("case %q: bad hex: %v", c.Name, err)
		}
		return b
	}
	return []byte(c.Text)
}

func loadCorpus(t *testing.T) (corpusFile, expectedFile) {
	t.Helper()
	var corpus corpusFile
	var expected expectedFile
	for _, l := range []struct {
		path string
		into any
	}{
		{"testdata/corpus.json", &corpus},
		{"testdata/expected.json", &expected},
	} {
		data, err := os.ReadFile(l.path)
		if err != nil {
			t.Fatalf("read %s: %v", l.path, err)
		}
		if err := json.Unmarshal(data, l.into); err != nil {
			t.Fatalf("parse %s: %v", l.path, err)
		}
	}
	if corpus.Encoding != expected.Encoding {
		t.Fatalf("encoding mismatch: corpus %q, expected %q", corpus.Encoding, expected.Encoding)
	}
	if len(corpus.Cases) != len(expected.Cases) {
		t.Fatalf("corpus has %d cases but expected.json has %d; rerun scripts/gen_expected.py",
			len(corpus.Cases), len(expected.Cases))
	}
	return corpus, expected
}

// TestConformanceTokenIDs is the contract: for every case in the corpus,
// tokdump must produce the same token IDs the OpenAI tokenizer does.
func TestConformanceTokenIDs(t *testing.T) {
	corpus, expected := loadCorpus(t)
	for _, c := range corpus.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			want, ok := expected.Cases[c.Name]
			if !ok {
				t.Fatalf("no expected entry; rerun scripts/gen_expected.py")
			}
			raw := c.bytes(t)

			text, valid := decodeUTF8(raw)
			if valid != want.ValidUTF8 {
				t.Errorf("valid UTF-8 = %v, want %v (%s)", valid, want.ValidUTF8, c.Note)
			}
			// Lock the decoder itself to Python's replacement behaviour, not
			// just the resulting IDs: a wrong number of U+FFFD can still land
			// on the right tokens by luck.
			if got := hex.EncodeToString([]byte(text)); got != want.DecodedHex {
				t.Errorf("decoded bytes mismatch (%s)\n got: %s\nwant: %s", c.Note, got, want.DecodedHex)
			}

			ids, pieces, err := tokenize(text)
			if err != nil {
				t.Fatalf("tokenize: %v", err)
			}
			if len(ids) != len(pieces) {
				t.Fatalf("got %d ids but %d pieces", len(ids), len(pieces))
			}
			if len(ids) != len(want.IDs) {
				t.Fatalf("got %d tokens, want %d (%s)\n got: %v\nwant: %v",
					len(ids), len(want.IDs), c.Note, ids, want.IDs)
			}
			for i := range ids {
				if ids[i] != want.IDs[i] {
					t.Errorf("token %d = %d, want %d (%s)", i, ids[i], want.IDs[i], c.Note)
				}
			}
		})
	}
}

// Token pieces must reassemble into exactly the decoded input. If they do not,
// the text column is showing something the tokenizer did not actually see.
func TestConformancePiecesReassemble(t *testing.T) {
	corpus, _ := loadCorpus(t)
	for _, c := range corpus.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			text, _ := decodeUTF8(c.bytes(t))
			_, pieces, err := tokenize(text)
			if err != nil {
				t.Fatalf("tokenize: %v", err)
			}
			var joined bytes.Buffer
			for _, p := range pieces {
				joined.WriteString(p)
			}
			if joined.String() != text {
				t.Errorf("pieces do not reassemble (%s)\n got: %x\nwant: %x",
					c.Note, joined.String(), text)
			}
		})
	}
}

// TestDumpGolden pins the rendered output for the whole corpus, so a change to
// the text column shows up as a reviewable diff rather than a surprise.
// Regenerate with: go test -run TestDumpGolden -update
func TestDumpGolden(t *testing.T) {
	corpus, _ := loadCorpus(t)
	const goldenPath = "testdata/dump_golden.txt"

	var got bytes.Buffer
	for _, c := range corpus.Cases {
		text, valid := decodeUTF8(c.bytes(t))
		fmt.Fprintf(&got, "=== %s\n", c.Name)
		fmt.Fprintf(&got, "--- %s\n", c.Note)
		if !valid {
			fmt.Fprintf(&got, "--- input is not valid UTF-8\n")
		}
		if err := dumpText(text, &got, false, newLayout(false)); err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		got.WriteByte('\n')
	}

	if *updateGolden {
		if err := os.WriteFile(goldenPath, got.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read %s (regenerate with -update): %v", goldenPath, err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("dump output changed; review and regenerate with:\n"+
			"  go test -run TestDumpGolden -update\n%s", firstDiff(want, got.Bytes()))
	}
}

// firstDiff reports the first differing line, which is far more useful than
// dumping two thousand lines of output.
func firstDiff(want, got []byte) string {
	wl := bytes.Split(want, []byte("\n"))
	gl := bytes.Split(got, []byte("\n"))
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if !bytes.Equal(wl[i], gl[i]) {
			return fmt.Sprintf("first difference at line %d:\n want: %q\n  got: %q", i+1, wl[i], gl[i])
		}
	}
	return fmt.Sprintf("line counts differ: want %d, got %d", len(wl), len(gl))
}
