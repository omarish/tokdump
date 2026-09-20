package main

import (
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	tiktokenloader "github.com/pkoukk/tiktoken-go-loader"
)

// defaultEncodingName is the encoding used by tokdump in v1.
// A future -e/--encoding flag will plug in here; for now we always return o200k_base.
const defaultEncodingName = "o200k_base"

var (
	encOnce sync.Once
	encVal  *tiktoken.Tiktoken
	encErr  error
)

// encoding returns the tokenizer for this run of tokdump.
// Isolated so a future CLI flag can choose among encodings without touching call sites.
func encoding() (*tiktoken.Tiktoken, error) {
	encOnce.Do(func() {
		// The offline loader keeps the vocabulary embedded in the binary, so
		// there are no network calls and no cache directory at runtime.
		tiktoken.SetBpeLoader(tiktokenloader.NewOfflineLoader())
		// v1: always o200k_base. Future: map -e/--encoding (or model name) here.
		encVal, encErr = tiktoken.GetEncoding(defaultEncodingName)
		if encErr != nil {
			encErr = fmt.Errorf("encoding %s: %w", defaultEncodingName, encErr)
		}
	})
	return encVal, encErr
}

// tokenize returns token IDs and the corresponding decoded token piece strings.
//
// A piece is the raw bytes behind one token, which need not be a whole
// character: an astral-plane emoji spans several tokens, so pieces can hold
// fragments of a UTF-8 sequence. Concatenating them reproduces the input
// exactly, which conformance_test.go asserts across the corpus.
func tokenize(text string) (ids []int, pieces []string, err error) {
	enc, err := encoding()
	if err != nil {
		return nil, nil, err
	}
	// EncodeOrdinary treats "<|endoftext|>" and friends as the literal text the
	// user actually typed, which is what tokdump is being asked to dump.
	ids = enc.EncodeOrdinary(text)
	pieces = make([]string, len(ids))
	for i, id := range ids {
		pieces[i] = enc.Decode([]int{id})
	}
	return ids, pieces, nil
}
