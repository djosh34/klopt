//nolint:godoclint // Public-seam test names are intentionally descriptive.
package stringlanguage_test

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/djosh34/klopt/pkg/internal/stringlanguage"
	"github.com/stretchr/testify/require"
)

func TestUUIDFormatsAreTheSameExactLanguage(t *testing.T) {
	t.Parallel()

	valid := []string{
		"a1234567-1234-4234-8234-123456789abc",
		"A1234567-1234-4234-B234-123456789ABC",
	}
	invalid := []string{
		"a1234567-1234-1234-8234-123456789abc",
		"a1234567-1234-4234-7234-123456789abc",
		"{a1234567-1234-4234-8234-123456789abc}",
	}

	for _, name := range []string{"uuid", "uuidv4", "uuid-v4"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			language, err := stringlanguage.Format(name)
			require.NoError(t, err)

			set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
				Language: language, WantMatch: true,
			}}, stringlanguage.Length{})
			require.NoError(t, err)

			for _, value := range valid {
				require.True(t, set.Matches(value), value)
			}

			for _, value := range invalid {
				require.False(t, set.Matches(value), value)
			}

			for seed := range uint64(100) {
				require.True(t, set.Matches(set.Generate(seed)))
			}
		})
	}
}

func TestByteIsStrictPaddedStandardBase64(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Format("byte")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: language, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	for _, value := range []string{"", "YQ==", "YWI=", "YWJj", "+///"} {
		require.True(t, set.Matches(value), value)
	}

	for _, value := range []string{"%%%", "YQ", "YQ=", "YR==", "YWJ="} {
		require.False(t, set.Matches(value), value)
	}

	for seed := range uint64(100) {
		value := set.Generate(seed)
		require.True(t, set.Matches(value))
		_, err := base64.StdEncoding.Strict().DecodeString(value)
		require.NoError(t, err, value)
	}
}

func TestIPv4IsCanonicalDottedDecimal(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Format("ipv4")
	require.NoError(t, err)

	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: language, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	for _, value := range []string{"0.0.0.0", "10.0.0.1", "255.255.255.255"} {
		require.True(t, set.Matches(value), value)
	}

	for _, value := range []string{"010.0.0.1", "256.0.0.1", "1.2.3", "1.2.3.4.5"} {
		require.False(t, set.Matches(value), value)
	}

	for seed := range uint64(100) {
		require.True(t, set.Matches(set.Generate(seed)))
	}
}

func TestCIDRAliasesAcceptAddressWithPrefixWithoutNormalization(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"cidr", "ipv4-cidr"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			language, err := stringlanguage.Format(name)
			require.NoError(t, err)

			set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
				Language: language, WantMatch: true,
			}}, stringlanguage.Length{})
			require.NoError(t, err)

			for _, value := range []string{"0.0.0.0/0", "192.0.2.0/24", "192.0.2.7/24", "255.255.255.255/32"} {
				require.True(t, set.Matches(value), value)
			}

			for _, value := range []string{"192.0.2.7", "192.0.2.7/33", "192.0.02.7/24"} {
				require.False(t, set.Matches(value), value)
			}
		})
	}

	format, err := stringlanguage.Format("cidr")
	require.NoError(t, err)
	pattern, err := stringlanguage.Pattern(`^192\.0\.2\.7/24$`)
	require.NoError(t, err)

	set, err := stringlanguage.Compile([]stringlanguage.Requirement{
		{Language: format, WantMatch: true},
		{Language: pattern, WantMatch: true},
	}, stringlanguage.Length{})
	require.NoError(t, err)
	require.Equal(t, "192.0.2.7/24", set.Generate(0))

	alias, err := stringlanguage.Format("ipv4-cidr")
	require.NoError(t, err)
	_, err = stringlanguage.Compile([]stringlanguage.Requirement{
		{Language: format, WantMatch: false},
		{Language: alias, WantMatch: true},
	}, stringlanguage.Length{})

	var empty *stringlanguage.EmptyError
	require.ErrorAs(t, err, &empty)
}

func TestDateMatchesRealCalendarDatesAndIntersectsPatterns(t *testing.T) {
	t.Parallel()

	format, err := stringlanguage.Format("date")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: format, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	for _, value := range []string{"0000-02-29", "1904-02-29", "2000-02-29", "2024-02-29", "2026-07-14"} {
		require.True(t, set.Matches(value), value)
	}

	for _, value := range []string{"1900-02-29", "2024-02-30", "2026-7-14", "2026-07-14Z"} {
		require.False(t, set.Matches(value), value)
	}

	pattern, err := stringlanguage.Pattern(`^2024-`)
	require.NoError(t, err)
	valid, err := stringlanguage.Compile([]stringlanguage.Requirement{
		{Language: format, WantMatch: true},
		{Language: pattern, WantMatch: true},
	}, stringlanguage.Length{})
	require.NoError(t, err)
	formatInvalid, err := stringlanguage.Compile([]stringlanguage.Requirement{
		{Language: format, WantMatch: false},
		{Language: pattern, WantMatch: true},
	}, stringlanguage.Length{})
	require.NoError(t, err)
	patternInvalid, err := stringlanguage.Compile([]stringlanguage.Requirement{
		{Language: format, WantMatch: true},
		{Language: pattern, WantMatch: false},
	}, stringlanguage.Length{})
	require.NoError(t, err)

	for seed := range uint64(100) {
		require.True(t, valid.Matches(valid.Generate(seed)))
		require.True(t, formatInvalid.Matches(formatInvalid.Generate(seed)))
		require.True(t, patternInvalid.Matches(patternInvalid.Generate(seed)))
	}
}

func TestDateTimeMatchesCurrentRFC3339ValidatorAndIntersectsPatterns(t *testing.T) {
	t.Parallel()

	format, err := stringlanguage.Format("date-time")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: format, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	for _, value := range []string{
		"2026-07-14T12:30:00Z",
		"2024-02-29T23:59:59.123Z",
		"2026-07-14T12:30:00,5+02:30",
	} {
		require.True(t, set.Matches(value), value)
	}

	for _, value := range []string{
		"2026-07-14Z",
		"2024-02-30T12:30:00Z",
		"2026-07-14t12:30:00z",
		"2026-07-14T24:00:00Z",
	} {
		require.False(t, set.Matches(value), value)
	}

	pattern, err := stringlanguage.Pattern(`Z$`)
	require.NoError(t, err)

	for _, requirements := range [][]stringlanguage.Requirement{
		{{Language: format, WantMatch: true}, {Language: pattern, WantMatch: true}},
		{{Language: format, WantMatch: false}, {Language: pattern, WantMatch: true}},
		{{Language: format, WantMatch: true}, {Language: pattern, WantMatch: false}},
	} {
		signed, err := stringlanguage.Compile(requirements, stringlanguage.Length{})
		require.NoError(t, err)

		for seed := range uint64(100) {
			require.True(t, signed.Matches(signed.Generate(seed)))
		}
	}
}

func TestEmailMatchesTheStaticRFC5321MailboxGrammar(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Format("email")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: language, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	valid := []string{
		"first.last@example.com",
		"!#$%&'*+-/=?^_`{|}~@example.com",
		`"John Doe"@example.com`,
		`"a b\\\"c"@example.com`,
		"postmaster@[192.0.2.1]",
		"postmaster@[IPv6:2001:db8::1]",
		"postmaster@[IPv6:::ffff:192.0.2.1]",
		"postmaster@[TAG:value]",
	}
	invalid := []string{
		"a..b@example.com",
		".a@example.com",
		"a@-example.com",
		"a@example-.com",
		"a@[256.0.0.1]",
		"a@[IPv6:2001:::1]",
		"a@[IPv6:not-an-ip]",
	}

	for _, value := range valid {
		require.True(t, set.Matches(value), value)
	}

	for _, value := range invalid {
		require.False(t, set.Matches(value), value)
	}

	for _, value := range valid {
		pattern, patternErr := stringlanguage.Pattern(`^` + regexp.QuoteMeta(value) + `$`)
		require.NoError(t, patternErr)

		exact, compileErr := stringlanguage.Compile([]stringlanguage.Requirement{
			{Language: language, WantMatch: true},
			{Language: pattern, WantMatch: true},
		}, stringlanguage.Length{})
		require.NoError(t, compileErr, value)
		require.Equal(t, value, exact.Generate(0))
	}

	for seed := range uint64(100) {
		require.True(t, set.Matches(set.Generate(seed)))
	}
}

func TestEmailEnforcesRFC5321MailboxPartSizeLimits(t *testing.T) {
	t.Parallel()

	language, err := stringlanguage.Format("email")
	require.NoError(t, err)
	set, err := stringlanguage.Compile([]stringlanguage.Requirement{{
		Language: language, WantMatch: true,
	}}, stringlanguage.Length{})
	require.NoError(t, err)

	require.True(t, set.Matches(strings.Repeat("a", 64)+"@x"))
	require.False(t, set.Matches(strings.Repeat("a", 65)+"@x"))
	require.True(t, set.Matches("a@"+strings.Repeat("b", 63)))
	require.False(t, set.Matches("a@"+strings.Repeat("b", 64)))

	domain252 := strings.Join([]string{
		strings.Repeat("a", 63),
		strings.Repeat("b", 63),
		strings.Repeat("c", 63),
		strings.Repeat("d", 60),
	}, ".")
	domain253 := domain252 + "d"
	require.Len(t, "a@"+domain252, 254)
	require.Len(t, "a@"+domain253, 255)
	require.True(t, set.Matches("a@"+domain252))
	require.False(t, set.Matches("a@"+domain253))
}
