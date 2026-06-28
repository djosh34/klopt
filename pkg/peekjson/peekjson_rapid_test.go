package peekjson

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// peekJSONRapidChecks is the default property-test iteration count.
const peekJSONRapidChecks = "50000"

// TestMain raises the default rapid check count unless the caller configured it.
func TestMain(m *testing.M) {
	if !rapidChecksExplicitlyConfigured() {
		if err := flag.Set("rapid.checks", peekJSONRapidChecks); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}

// rapidChecksExplicitlyConfigured reports whether rapid.checks came from env or flags.
func rapidChecksExplicitlyConfigured() bool {
	if _, ok := os.LookupEnv("RAPID_CHECKS"); ok {
		return true
	}

	wasSet := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "rapid.checks" {
			wasSet = true
		}
	})

	return wasSet
}

// rapidDecoderOp identifies an operation to compare across decoder implementations.
type rapidDecoderOp uint8

const (
	// rapidOpDecode decodes the next JSON value into an interface value.
	rapidOpDecode rapidDecoderOp = iota
	// rapidOpToken reads the next JSON token.
	rapidOpToken
	// rapidOpMore asks whether the decoder has another element in the current array or object.
	rapidOpMore
	// rapidOpInputOffset reads the decoder's current byte offset.
	rapidOpInputOffset
)

// TestRapidDecoderMatchesEncodingJSON compares peekjson.Decoder with encoding/json.Decoder.
func TestRapidDecoderMatchesEncodingJSON(t *testing.T) {
	t.Parallel()

	var (
		numValid        int
		numValidObjects int
		numInvalid      int
	)

	rapid.Check(t, func(rt *rapid.T) {
		input := drawChaoticJSONStream(rt)

		var testValid any

		err := json.Unmarshal([]byte(input), &testValid)
		if err == nil {
			numValid++
		} else {
			numInvalid++
		}

		var testValidObject map[string]json.RawMessage

		err = json.Unmarshal([]byte(input), &testValidObject)
		if err == nil {
			numValidObjects++
		}

		useNumber := rapid.Bool().Draw(rt, "use number")
		opCount := rapid.IntRange(1, 80).Draw(rt, "op count")

		want := json.NewDecoder(strings.NewReader(input))
		got := NewDecoder(strings.NewReader(input))

		if useNumber {
			want.UseNumber()
			got.UseNumber()
		}

		for range opCount {
			op := drawRapidDecoderOp(rt, "draw Operation")

			requireRapidDecoderOpMatches(t, want, got, op)
			requireRepeatedPeeksMatch(t, rt, got)
		}
	})

	t.Logf("Num valid: %d Num invalid: %d", numValid, numInvalid)
	t.Logf("Num valid objects: %d", numValidObjects)

	require.NotEqualf(t, 0, numValid, "only zero valid json inputs")
	require.NotEqualf(t, 0, numValidObjects, "only zero valid json objects")
}

// requireRapidDecoderOpMatches checks one operation against encoding/json.Decoder.
func requireRapidDecoderOpMatches(t *testing.T, want *json.Decoder, got *Decoder, op rapidDecoderOp) {
	t.Helper()

	switch op {
	case rapidOpDecode:
		requireRapidDecodeMatches(t, want, got)
	case rapidOpToken:
		requireRapidTokenMatches(t, want, got)
	case rapidOpMore:
		require.Equal(t, want.More(), got.More())
	case rapidOpInputOffset:
		require.Equal(t, want.InputOffset(), got.InputOffset())
	default:
		t.Fatalf("unknown op %d", op)
	}
}

// requireRapidDecodeMatches decodes one value from both decoders and compares results.
func requireRapidDecodeMatches(t *testing.T, want *json.Decoder, got *Decoder) {
	t.Helper()

	var (
		wantValue any
		gotValue  any
	)

	wantErr := want.Decode(&wantValue)
	gotErr := got.Decode(&gotValue)

	require.Equal(t, wantErr, gotErr)
	require.Equal(t, wantValue, gotValue)
}

// requireRapidTokenMatches reads one token from both decoders and compares results.
func requireRapidTokenMatches(t *testing.T, want *json.Decoder, got *Decoder) {
	t.Helper()

	wantTok, wantErr := want.Token()
	gotTok, gotErr := got.Token()

	require.Equal(t, wantErr, gotErr)
	require.Equal(t, wantTok, gotTok)
}

// requireRepeatedPeeksMatch verifies that repeated peeks return stable results.
func requireRepeatedPeeksMatch(t *testing.T, rt *rapid.T, got *Decoder) {
	t.Helper()

	shouldPeek := rapid.Bool().Draw(rt, "should peek")
	if !shouldPeek {
		return
	}

	peekedToken, peekErr := got.Peek()

	peekCount := rapid.IntRange(0, 80).Draw(rt, "additional peek count")
	for range peekCount {
		additionalPeekToken, additionalErr := got.Peek()

		require.Equal(t, peekErr, additionalErr)
		require.Equal(t, peekedToken, additionalPeekToken)
	}
}

// drawRapidDecoderOp draws a weighted decoder operation for the property test.
func drawRapidDecoderOp(t *rapid.T, label string) rapidDecoderOp {
	switch rapid.IntRange(0, 9).Draw(t, label+" kind") {
	case 0, 1, 2, 3:
		return rapidOpDecode
	case 4, 5, 6, 7:
		return rapidOpToken
	case 8:
		return rapidOpMore
	default:
		return rapidOpInputOffset
	}
}

// drawChaoticJSONStream draws a JSON stream and may corrupt it in controlled ways.
func drawChaoticJSONStream(t *rapid.T) string {
	maxDepth := rapid.IntRange(0, 5).Draw(t, "max depth")
	valueCount := rapid.IntRange(1, 6).Draw(t, "top-level value count")

	var b strings.Builder
	mustWriteString(t, &b, drawJSONWhitespace(t, "stream prefix whitespace"))

	for i := range valueCount {
		mustWriteString(t, &b, drawJSONValue(t, maxDepth, fmt.Sprintf("root %d", i)))
		mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("root %d suffix whitespace", i)))
	}

	input := b.String()

	switch rapid.IntRange(0, 9).Draw(t, "chaos mode") {
	case 0:
		cut := rapid.IntRange(0, len(input)).Draw(t, "truncate position")

		return input[:cut]
	case 1:
		pos := rapid.IntRange(0, len(input)).Draw(t, "insert junk position")

		return input[:pos] + drawJSONJunk(t, "insert junk") + input[pos:]
	case 2:
		return input + drawJSONJunk(t, "append junk")
	case 3:
		if len(input) == 0 {
			return input
		}

		start := rapid.IntRange(0, len(input)-1).Draw(t, "delete start")
		end := rapid.IntRange(start+1, len(input)).Draw(t, "delete end")

		return input[:start] + input[end:]
	default:
		return input
	}
}

// drawJSONValue draws any JSON value allowed at the requested nesting depth.
func drawJSONValue(t *rapid.T, depth int, label string) string {
	maxKind := 3
	if depth > 0 {
		maxKind = 5
	}

	switch rapid.IntRange(0, maxKind).Draw(t, label+" kind") {
	case 0:
		return "null"
	case 1:
		if rapid.Bool().Draw(t, label+" bool") {
			return "true"
		}

		return "false"
	case 2:
		return drawJSONStringLiteral(t, label+" string")
	case 3:
		return drawJSONNumber(t, label+" number")
	case 4:
		return drawJSONArray(t, depth, label+" array")
	default:
		return drawJSONObject(t, depth, label+" object")
	}
}

// drawJSONArray draws a JSON array with recursively generated elements.
func drawJSONArray(t *rapid.T, depth int, label string) string {
	length := rapid.IntRange(0, 8).Draw(t, label+" len")

	var b strings.Builder
	mustWriteByte(t, &b, '[')
	mustWriteString(t, &b, drawJSONWhitespace(t, label+" open whitespace"))

	for i := range length {
		if i > 0 {
			mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s comma %d left whitespace", label, i)))
			mustWriteByte(t, &b, ',')
			mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s comma %d right whitespace", label, i)))
		}

		mustWriteString(t, &b, drawJSONValue(t, depth-1, fmt.Sprintf("%s item %d", label, i)))
	}

	mustWriteString(t, &b, drawJSONWhitespace(t, label+" close whitespace"))
	mustWriteByte(t, &b, ']')

	return b.String()
}

// drawJSONObject draws a JSON object with generated keys and values.
func drawJSONObject(t *rapid.T, depth int, label string) string {
	length := rapid.IntRange(0, 8).Draw(t, label+" len")

	var b strings.Builder
	mustWriteByte(t, &b, '{')
	mustWriteString(t, &b, drawJSONWhitespace(t, label+" open whitespace"))

	for i := range length {
		if i > 0 {
			mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s comma %d left whitespace", label, i)))
			mustWriteByte(t, &b, ',')
			mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s comma %d right whitespace", label, i)))
		}

		mustWriteString(t, &b, drawJSONStringLiteral(t, fmt.Sprintf("%s key %d", label, i)))
		mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s colon %d left whitespace", label, i)))
		mustWriteByte(t, &b, ':')
		mustWriteString(t, &b, drawJSONWhitespace(t, fmt.Sprintf("%s colon %d right whitespace", label, i)))
		mustWriteString(t, &b, drawJSONValue(t, depth-1, fmt.Sprintf("%s value %d", label, i)))
	}

	mustWriteString(t, &b, drawJSONWhitespace(t, label+" close whitespace"))
	mustWriteByte(t, &b, '}')

	return b.String()
}

// drawJSONStringLiteral draws a quoted JSON string.
func drawJSONStringLiteral(t *rapid.T, label string) string {
	length := rapid.IntRange(0, 32).Draw(t, label+" len")

	var b strings.Builder
	mustWriteByte(t, &b, '"')

	for i := range length {
		mustWriteString(t, &b, drawJSONStringSegment(t, fmt.Sprintf("%s segment %d", label, i)))
	}

	mustWriteByte(t, &b, '"')

	return b.String()
}

// drawJSONStringSegment draws one escaped or raw segment for a JSON string.
func drawJSONStringSegment(t *rapid.T, label string) string {
	switch rapid.IntRange(0, 7).Draw(t, label+" kind") {
	case 0:
		return `\"`
	case 1:
		return `\\`
	case 2:
		return rapid.SampledFrom([]string{`\/`, `\b`, `\f`, `\n`, `\r`, `\t`}).
			Draw(t, label+" escaped control")
	case 3:
		return fmt.Sprintf(`\u%04x`, rapid.Uint16().Draw(t, label+" unicode escape"))
	case 4:
		hi := rapid.Uint16Range(0xd800, 0xdbff).Draw(t, label+" high surrogate")
		lo := rapid.Uint16Range(0xdc00, 0xdfff).Draw(t, label+" low surrogate")

		return fmt.Sprintf(`\u%04x\u%04x`, hi, lo)
	default:
		return string(drawJSONSafeStringByte(t, label+" raw byte"))
	}
}

// drawJSONSafeStringByte draws an ASCII byte that needs no JSON string escaping.
func drawJSONSafeStringByte(t *rapid.T, label string) byte {
	for {
		b := rapid.ByteRange(0x20, 0x7e).Draw(t, label)
		if b != '"' && b != '\\' {
			return b
		}
	}
}

// drawJSONNumber draws a JSON number with optional fraction and exponent parts.
func drawJSONNumber(t *rapid.T, label string) string {
	var b strings.Builder
	if rapid.Bool().Draw(t, label+" negative") {
		mustWriteByte(t, &b, '-')
	}

	if rapid.Bool().Draw(t, label+" zero integer") {
		mustWriteByte(t, &b, '0')
	} else {
		mustWriteByte(t, &b, rapid.ByteRange('1', '9').Draw(t, label+" first digit"))
		mustWriteString(t, &b, drawJSONDigits(t, 0, 40, label+" integer tail"))
	}

	if rapid.Bool().Draw(t, label+" has fraction") {
		mustWriteByte(t, &b, '.')
		mustWriteString(t, &b, drawJSONDigits(t, 1, 40, label+" fraction"))
	}

	if rapid.Bool().Draw(t, label+" has exponent") {
		mustWriteByte(t, &b, rapid.SampledFrom([]byte{'e', 'E'}).Draw(t, label+" exponent marker"))

		if rapid.Bool().Draw(t, label+" exponent sign") {
			mustWriteByte(t, &b, rapid.SampledFrom([]byte{'+', '-'}).Draw(t, label+" exponent sign byte"))
		}

		if rapid.IntRange(0, 4).Draw(t, label+" exponent chaos") == 0 {
			mustWriteString(t, &b, rapid.SampledFrom([]string{"309", "400", "9999", "000001"}).
				Draw(t, label+" large exponent"))
		} else {
			mustWriteString(t, &b, drawJSONDigits(t, 1, 8, label+" exponent"))
		}
	}

	return b.String()
}

// drawJSONDigits draws a decimal digit sequence within the requested length range.
func drawJSONDigits(t *rapid.T, minLen int, maxLen int, label string) string {
	length := rapid.IntRange(minLen, maxLen).Draw(t, label+" len")

	var b strings.Builder
	for i := range length {
		mustWriteByte(t, &b, rapid.ByteRange('0', '9').Draw(t, fmt.Sprintf("%s digit %d", label, i)))
	}

	return b.String()
}

// drawJSONWhitespace draws JSON whitespace bytes.
func drawJSONWhitespace(t *rapid.T, label string) string {
	length := rapid.IntRange(0, 8).Draw(t, label+" len")

	var b strings.Builder
	for i := range length {
		mustWriteByte(t, &b, rapid.SampledFrom([]byte{' ', '\n', '\r', '\t'}).
			Draw(t, fmt.Sprintf("%s byte %d", label, i)))
	}

	return b.String()
}

// drawJSONJunk draws non-JSON bytes for corrupting otherwise valid streams.
func drawJSONJunk(t *rapid.T, label string) string {
	length := rapid.IntRange(1, 8).Draw(t, label+" len")

	var b strings.Builder
	for i := range length {
		mustWriteByte(t, &b, rapid.Byte().Draw(t, fmt.Sprintf("%s byte %d", label, i)))
	}

	return b.String()
}

// mustWriteString appends s to b and panics if strings.Builder reports an error.
func mustWriteString(t *rapid.T, b *strings.Builder, s string) {
	if _, err := b.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// mustWriteByte appends c to b and panics if strings.Builder reports an error.
func mustWriteByte(t *rapid.T, b *strings.Builder, c byte) {
	if err := b.WriteByte(c); err != nil {
		t.Fatal(err)
	}
}
