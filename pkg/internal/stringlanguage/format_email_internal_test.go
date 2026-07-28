//nolint:godoclint // Internal grammar regression has a descriptive test name.
package stringlanguage

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailPatternReservesIPv6TagForIPv6Syntax(t *testing.T) {
	t.Parallel()

	compiled := regexp.MustCompile(emailPattern())
	snum := `([0-9]|[0-9]{2}|[01][0-9]{2}|2[0-4][0-9]|25[0-5])`
	ipv4 := snum + `\.` + snum + `\.` + snum + `\.` + snum
	require.False(t, regexp.MustCompile(`^`+ipv6AddressPattern(ipv4)+`$`).MatchString("2001:::1"))
	require.False(t, regexp.MustCompile(`^`+generalAddressLiteralPattern()+`$`).MatchString("IPv6:2001:::1"))
	require.False(t, regexp.MustCompile(`^`+generalAddressLiteralPattern()+`$`).MatchString("ipv6:2001:::1"))
	require.True(t, compiled.MatchString("a@[IPv6:2001:db8::1]"))
	require.True(t, compiled.MatchString("a@[ipv6:2001:db8::1]"))
	require.True(t, compiled.MatchString("a@[TAG:value]"))
	require.False(t, compiled.MatchString("a@[IPv6:2001:::1]"))
}
