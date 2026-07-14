//nolint:godoclint,mnd // Private catalog layout ordinals are part of the verified data recipe.
package testgenerator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	opaqueFamilyCount        = 5
	opaqueFragmentsPerFamily = 4
	opaqueCommonCount        = 40
	opaqueUniqueCount        = 70
)

type opaqueStringFragment struct {
	Pattern         string
	Format          string
	ValidExamples   []json.RawMessage
	InvalidExamples []json.RawMessage
}

var opaqueStringCatalog = buildOpaqueStringCatalog()

func buildOpaqueStringCatalog() []opaqueStringFragment {
	catalog := make([]opaqueStringFragment, 0, opaqueFamilyCount*opaqueFragmentsPerFamily)

	for family := 0; family < opaqueFamilyCount; family++ {
		for fragment := 0; fragment < opaqueFragmentsPerFamily; fragment++ {
			valid := make([]json.RawMessage, 0, opaqueCommonCount+opaqueUniqueCount)
			invalid := make([]json.RawMessage, 0, opaqueCommonCount+opaqueUniqueCount)

			for index := 0; index < opaqueCommonCount; index++ {
				valid = append(valid, opaqueRawString(opaqueValidString(family, index)))
				invalid = append(invalid, opaqueRawString(fmt.Sprintf("!shared-%d-%03d", family, index)))
			}

			for index := 0; index < opaqueUniqueCount; index++ {
				ordinal := 100 + fragment*opaqueUniqueCount + index
				valid = append(valid, opaqueRawString(opaqueValidString(family, ordinal)))
				invalid = append(invalid, opaqueRawString(
					fmt.Sprintf("!unique-%d-%d-%03d", family, fragment, index),
				))
			}

			pattern, format := opaquePatternAndFormat(family)
			catalog = append(catalog, opaqueStringFragment{
				Pattern: pattern, Format: format, ValidExamples: valid, InvalidExamples: invalid,
			})
		}
	}

	return catalog
}

func opaqueRawString(value string) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("encode trusted opaque string: %w", err))
	}

	return raw
}

func opaqueValidString(family int, ordinal int) string {
	switch family {
	case 0:
		return fmt.Sprintf("C%03d", ordinal)
	case 1:
		return fmt.Sprintf("u%03d@example.com", ordinal)
	case 2:
		return time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, ordinal).Format(time.DateOnly)
	case 3:
		return time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC).
			Add(time.Duration(ordinal) * time.Second).Format(time.RFC3339)
	case 4:
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("value-%03d", ordinal)))
	default:
		panic(fmt.Sprintf("unknown opaque family %d", family))
	}
}

func opaquePatternAndFormat(family int) (string, string) {
	switch family {
	case 0:
		return `^C[0-9]{3}$`, ""
	case 1:
		return `^u[0-9]{3}@example[.]com$`, "email"
	case 2:
		return `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`, "date"
	case 3:
		return `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`, "date-time"
	case 4:
		return `^[A-Za-z0-9+/]+={0,2}$`, "byte"
	default:
		panic(fmt.Sprintf("unknown opaque family %d", family))
	}
}
