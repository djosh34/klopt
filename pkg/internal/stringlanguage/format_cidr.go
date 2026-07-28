//nolint:godoclint // Private format construction stays behind Format.
package stringlanguage

func cidrLanguage() (Language, error) {
	prefix := `(0|[1-9]|[12][0-9]|3[0-2])`

	return formatPattern(`^` + ipv4OctetPattern + `\.` + ipv4OctetPattern + `\.` +
		ipv4OctetPattern + `\.` + ipv4OctetPattern + `/` + prefix + `$`)
}
