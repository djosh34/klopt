// Package types defines shared test-generator domain contracts.
package types

// AllOfMerger merges another domain into an allOf intersection.
type AllOfMerger interface {
	// AllOfMerge intersects the receiver with domain.
	AllOfMerge(domain Domain) (Domain, error)
}

// Domain is a hashable schema domain that supports allOf merging.
type Domain interface {
	Hasher
	AllOfMerger
}

// Hash is a SHA-256 domain hash.
type Hash [32]byte

// Hasher produces a deterministic domain hash.
type Hasher interface {
	// GenerateHash returns a deterministic hash of the receiver.
	GenerateHash() (Hash, error)
}
