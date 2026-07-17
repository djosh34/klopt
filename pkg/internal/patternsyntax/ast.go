// Package patternsyntax parses the supported ECMAScript 5.1 regular-expression syntax.
//
// The package owns syntax, source spans, and parser limits. It deliberately has no
// matching or generation behavior; callers translate Tree data independently.
package patternsyntax

// Parser resource limits.
const (
	MaximumSourceBytes       = 64 * 1024
	MaximumNestingDepth      = 100
	MaximumNodes             = 10_000
	MaximumLeadingAssertions = 64
	MaximumRepeatEndpoint    = 1_000
	MaximumRepeatProduct     = 1_000
)

// NodeID identifies a node in Tree.Nodes.
type NodeID int

// Kind identifies one syntax-only AST node kind.
type Kind uint8

// AST node kinds.
const (
	KindExpression Kind = iota
	KindAlternative
	KindLiteral
	KindDot
	KindClass
	KindDigit
	KindNotDigit
	KindSpace
	KindNotSpace
	KindWord
	KindNotWord
	KindBeginInput
	KindEndInput
	KindWordBoundary
	KindNotWordBoundary
	KindPositiveLookahead
	KindNegativeLookahead
	KindCapture
	KindGroup
	KindRepeat
)

// Span is one half-open UTF-8 byte range in the original source.
type Span struct {
	Start int
	End   int
}

// Repeat describes a quantified child.
type Repeat struct {
	Minimum   int
	Maximum   int
	Unbounded bool
	Lazy      bool
	Counted   bool
}

// ClassItemKind identifies one character-class member representation.
type ClassItemKind uint8

// Character-class item kinds.
const (
	ClassItemRange ClassItemKind = iota
	ClassItemDigit
	ClassItemNotDigit
	ClassItemSpace
	ClassItemNotSpace
	ClassItemWord
	ClassItemNotWord
)

// ClassItem is one character range or predefined character set.
type ClassItem struct {
	Kind ClassItemKind
	Low  rune
	High rune
}

// Node is one syntax-only AST node.
type Node struct {
	Kind       Kind
	Span       Span
	Children   []NodeID
	Repeat     Repeat
	Value      rune
	Negated    bool
	ClassItems []ClassItem
}

// Tree is one parsed pattern. Root indexes Nodes.
type Tree struct {
	Root  NodeID
	Nodes []Node
}
