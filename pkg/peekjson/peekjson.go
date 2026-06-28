// Package peekjson provides a JSON decoder with single-token lookahead.
package peekjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Decoder wraps json.Decoder with support for peeking at the next token.
type Decoder struct {
	json.Decoder

	sourceReader    io.Reader
	lookAheadBuffer bytes.Buffer

	peekDec     *json.Decoder
	peekedToken *json.Token
	peekedErr   error
	peeked      bool
}

// NewDecoder returns a decoder that reads JSON values from r.
func NewDecoder(r io.Reader) *Decoder {
	d := &Decoder{sourceReader: r}

	d.Decoder = *json.NewDecoder(publicReader{d})
	d.peekDec = json.NewDecoder(peekReader{d})
	d.peekDec.UseNumber()

	return d
}

// Peek returns the next JSON token without discarding its source bytes.
func (d *Decoder) Peek() (*json.Token, error) {
	if d.peeked {
		return d.peekedToken, d.peekedErr
	}

	tok, err := d.peekDec.Token()

	d.peeked = true
	if err != nil {
		d.peekedErr = err

		return nil, err
	}

	d.peekedToken = &tok

	return &tok, err
}

// clearPeek discards any cached lookahead state after the public decoder reads.
func (d *Decoder) clearPeek() {
	d.peeked = false
	d.peekedToken = nil
	d.peekedErr = nil
}

// peekReader reads from the source while saving bytes for the public decoder.
type peekReader struct {
	d *Decoder
}

// Read reads source bytes into p and mirrors them into the lookahead buffer.
func (r peekReader) Read(p []byte) (int, error) {
	n, err := r.d.sourceReader.Read(p)
	if n > 0 {
		written, writeErr := r.d.lookAheadBuffer.Write(p[:n])
		if writeErr != nil {
			return n, writeErr
		}

		if written != n {
			return n, io.ErrShortWrite
		}
	}

	return n, err
}

// publicReader replays lookahead bytes before reading from the source.
type publicReader struct {
	d *Decoder
}

// Read reads bytes from the lookahead buffer and then from the source reader.
func (r publicReader) Read(p []byte) (int, error) {
	d := r.d

	d.peekedToken = nil
	d.peekedErr = nil

	if len(p) == 0 {
		return 0, nil
	}

	d.clearPeek()

	n, err := d.lookAheadBuffer.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}

	if n == len(p) {
		return n, nil
	}

	m, err := d.sourceReader.Read(p[n:])

	return n + m, err
}
