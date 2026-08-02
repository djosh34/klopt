//nolint:godoclint // Private clean-model vocabulary stays behind parseInput and Build.
package schematest

// schemaKind is an admitted OAS 3.0 Schema Object type. schemaAny means no
// explicit type was authored.
type schemaKind uint8

const (
	schemaAny schemaKind = iota
	schemaBoolean
	schemaInteger
	schemaNumber
	schemaString
	schemaArray
	schemaObject
)

// schemaOccurrence identifies where a schema is used, where a Reference Object
// resolves, and which request instance location it describes.
type schemaOccurrence struct {
	usePointer       string
	targetPointer    string
	instanceTemplate string
}

type exactCount struct {
	number *exactNumber
}

func (count *exactCount) String() string {
	if count == nil || count.number == nil {
		return "<nil>"
	}

	value, err := count.number.canonicalDecimal()
	if err != nil {
		return "<invalid>"
	}

	return value
}

type schemaFormat uint8

const (
	schemaFormatNone schemaFormat = iota
	schemaFormatInt32
	schemaFormatInt64
	schemaFormatFloat
	schemaFormatDouble
	schemaFormatByte
	schemaFormatDate
	schemaFormatDateTime
	schemaFormatEmail
	schemaFormatIPv4
	schemaFormatUUID
	schemaFormatUUIDv4
	schemaFormatUUIDDashV4
	schemaFormatCIDR
	schemaFormatIPv4CIDR
	schemaFormatPassword
)

// schemaNode is the private clean-room representation of one Schema Object
// occurrence.
type schemaNode struct {
	occurrence   schemaOccurrence
	kind         schemaKind
	nullable     bool
	enum         []*jsonValue
	defaultValue *jsonValue

	minimum          *exactNumber
	maximum          *exactNumber
	multipleOf       *exactNumber
	exclusiveMinimum bool
	exclusiveMaximum bool
	format           schemaFormat

	minLength *exactCount
	maxLength *exactCount
	pattern   *patternAST

	minItems *exactCount
	maxItems *exactCount
	items    *schemaNode

	minProperties             *exactCount
	maxProperties             *exactCount
	required                  []string
	properties                map[string]*schemaNode
	allowAdditionalProperties bool
	additionalProperties      *schemaNode

	allOf []*schemaNode
	anyOf []*schemaNode

	readOnly  bool
	writeOnly bool
}

// schemaModel is the selected application/json request body's private model.
type schemaModel struct {
	root *schemaNode
}
