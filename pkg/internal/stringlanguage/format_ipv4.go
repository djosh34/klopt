//nolint:godoclint // Private format construction stays behind Format.
package stringlanguage

const ipv4OctetPattern = `(0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])`

func ipv4Language() (Language, error) {
	return formatPattern(`^` + ipv4OctetPattern + `\.` + ipv4OctetPattern + `\.` +
		ipv4OctetPattern + `\.` + ipv4OctetPattern + `$`)
}
