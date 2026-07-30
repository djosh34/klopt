//nolint:godoclint // Small exact-value helpers are shared by primitive theories.
package program

import "github.com/djosh34/klopt/pkg/jsonvalue"

func exactKindCount(values []jsonvalue.Value, kind jsonvalue.Kind) int {
	count := 0

	for _, value := range values {
		if value.Kind == kind {
			count++
		}
	}

	return count
}
