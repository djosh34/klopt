//nolint:godoclint // Private format construction stays behind Format.
package stringlanguage

const (
	leapYearPattern = `([0-9]{2}(0[48]|[2468][048]|[13579][26])|([02468][048]|[13579][26])00)`
	datePattern     = `([0-9]{4}-((01|03|05|07|08|10|12)-(0[1-9]|[12][0-9]|3[01])|` +
		`(04|06|09|11)-(0[1-9]|[12][0-9]|30)|02-(0[1-9]|1[0-9]|2[0-8]))|` +
		leapYearPattern + `-02-29)`
)

func dateLanguage() (Language, error) {
	return formatPattern(`^` + datePattern + `$`)
}

func dateTimeLanguage() (Language, error) {
	timePattern := `(0[0-9]|1[0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]([.,][0-9]+)?`
	offsetPattern := `(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])`

	return formatPattern(`^` + datePattern + `T` + timePattern + offsetPattern + `$`)
}
