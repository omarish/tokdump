package main

import "testing"

func TestEncodingIsO200k(t *testing.T) {
	enc, err := encoding()
	if err != nil {
		t.Fatal(err)
	}
	if got := enc.GetName(); got != "o200k_base" {
		t.Fatalf("encoding name = %q, want o200k_base", got)
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
