package peekjson

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicReaderRead(t *testing.T) {
	for name, tt := range map[string]struct {
		bytesToRead          int
		internalBuffer       string
		upstreamBuffer       string
		expectedOutputBuffer bytes.Buffer
		expectedBytesRead    int
		expectedErr          error
	}{
		"fully from internal buffer": {
			bytesToRead:    len("buffer"),
			internalBuffer: "buffer",
			upstreamBuffer: "upstream",
			expectedErr:    nil,
		},
		"fully from upstream": {
			bytesToRead:    len("up"),
			internalBuffer: "",
			upstreamBuffer: "upstream",
			expectedErr:    nil,
		},
		"fills from internal buffer then partial upstream": {
			bytesToRead:    len("bufst"),
			internalBuffer: "buf",
			upstreamBuffer: "stream",
			expectedErr:    nil,
		},
		"drains internal buffer and upstream before eof": {
			bytesToRead:    len("bufstream") + 10,
			internalBuffer: "buf",
			upstreamBuffer: "stream",
			expectedErr:    io.EOF,
		},
		"drains upstream before eof": {
			bytesToRead:    len("upstream") + 10,
			internalBuffer: "",
			upstreamBuffer: "upstream",
			expectedErr:    io.EOF,
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			upstream := strings.NewReader(tt.upstreamBuffer)

			d := &Decoder{sourceReader: upstream}
			d.lookAheadBuffer = *bytes.NewBufferString(tt.internalBuffer)

			p := make([]byte, tt.bytesToRead)

			// Act
			n, err := publicReader{d: d}.Read(p)

			// Assert
			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.expectedErr)
			}

			require.Equal(t, n, tt.expectedBytesRead)
			require.Equal(t, p, tt.expectedOutputBuffer)

		})
	}
}

func TestPeekReaderBuffersReadBytes(t *testing.T) {
	upstream := strings.NewReader("peeked")
	d := &Decoder{sourceReader: upstream}
	p := make([]byte, len("peek"))

	n, err := peekReader{d: d}.Read(p)

	require.NoError(t, err)
	require.Equal(t, len("peek"), n)
	require.Equal(t, "peek", string(p[:n]))
	require.Equal(t, "peek", d.lookAheadBuffer.String())
	require.Equal(t, "ed", remainingString(t, upstream))
}

func TestPeekReaderBuffersBytesReturnedWithError(t *testing.T) {
	readErr := errors.New("read failed")
	d := &Decoder{sourceReader: errorAfterBytesReader{data: "peek", err: readErr}}
	p := make([]byte, len("peek"))

	n, err := peekReader{d: d}.Read(p)

	require.ErrorIs(t, err, readErr)
	require.Equal(t, len("peek"), n)
	require.Equal(t, "peek", string(p[:n]))
	require.Equal(t, "peek", d.lookAheadBuffer.String())
}

func remainingString(t *testing.T, r io.Reader) string {
	t.Helper()

	remaining, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(remaining)
}

type errorAfterBytesReader struct {
	data string
	err  error
}

func (r errorAfterBytesReader) Read(p []byte) (int, error) {
	return copy(p, r.data), r.err
}
