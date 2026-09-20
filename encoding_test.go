package main

import "testing"

// The tokenizer library does not expose the encoding name, so pin the encoding
// by behaviour instead: these IDs are o200k_base and nothing else.
func TestEncodingIsO200k(t *testing.T) {
	enc, err := encoding()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		text string
		want []int
	}{
		{"Hello world", []int{13225, 2375}},
		{"hello", []int{24912}},
		// A special-token marker must count as the literal text it is.
		{"<|endoftext|>", []int{27, 91, 419, 1440, 919, 91, 29}},
	} {
		got := enc.EncodeOrdinary(c.text)
		if len(got) != len(c.want) {
			t.Errorf("EncodeOrdinary(%q) = %v, want %v", c.text, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("EncodeOrdinary(%q) = %v, want %v", c.text, got, c.want)
				break
			}
		}
	}
}

func TestTokenizeHelloWorld(t *testing.T) {
	ids, pieces, err := tokenize("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2; ids=%v pieces=%v", len(ids), ids, pieces)
	}
	if ids[0] != 13225 || ids[1] != 2375 {
		t.Fatalf("ids = %v, want [13225 2375]", ids)
	}
	if len(pieces) != 2 {
		t.Fatalf("len(pieces) = %d, want 2", len(pieces))
	}
	if pieces[0] != "Hello" || pieces[1] != " world" {
		t.Fatalf("pieces = %#v, want [Hello] [ world]", pieces)
	}
}

func TestTokenizeEmpty(t *testing.T) {
	ids, pieces, err := tokenize("")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 || len(pieces) != 0 {
		t.Fatalf("tokenize(\"\") = %v %v, want empty", ids, pieces)
	}
}
