//nolint:godoclint // Private format construction stays behind Format.
package stringlanguage

func byteLanguage() (Language, error) {
	alphabet := `[A-Za-z0-9+/]`

	return formatPattern(`^(` + alphabet + `{4})*(` + alphabet + `[AQgw]==|` +
		alphabet + `{2}[AEIMQUYcgkosw048]=)?$`)
}
