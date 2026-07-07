package types

type AllOfMerger interface {
	AllOfMerge(domain Domain) (Domain, error)
}

type ToHasher interface {
	ToHasher() (Hasher, error)
}

type Domain interface {
	ToHasher
	// Hasher
	AllOfMerger
}

type Hash [32]byte

type Hasher interface {
	GenerateHash() (Hash, error)
}
