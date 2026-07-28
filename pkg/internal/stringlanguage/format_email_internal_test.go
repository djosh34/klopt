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
	ipv4 := ipv4OctetPattern + `\.` + ipv4OctetPattern + `\.` + ipv4OctetPattern + `\.` + ipv4OctetPattern
	require.False(t, regexp.MustCompile(`^`+ipv6AddressPattern(ipv4)+`$`).MatchString("2001:::1"))
	require.False(t, regexp.MustCompile(`^`+generalAddressLiteralPattern()+`$`).MatchString("IPv6:2001:::1"))
	require.True(t, compiled.MatchString("a@[TAG:value]"))
	require.False(t, compiled.MatchString("a@[IPv6:2001:::1]"))
}
