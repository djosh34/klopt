//nolint:godoclint // Private seed helpers implement the fixed string-search identity.
package schematest

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
)

const (
	stringSeedBytes                 = 8
	stringSeedIntervalShift         = 16
	stringSeedFirstMixShift         = 29
	stringSeedSecondMixShift        = 32
	stringSeedMixMultiplier  uint64 = 0x9e3779b97f4a7c15
)

func stringSearchSeed(objective stringObjective) (uint64, error) {
	if err := validateStringSeedObjective(objective); err != nil {
		return 0, err
	}

	hasher := sha256.New()
	if err := writeStringSeedPrefix(hasher, objective); err != nil {
		return 0, err
	}

	if err := writeCanonicalJSON(hasher, objective.owner.source); err != nil {
		return 0, err
	}

	if err := writeStringSeedSuffix(hasher, objective); err != nil {
		return 0, err
	}

	digest := hasher.Sum(nil)
	if len(digest) < stringSeedBytes {
		return 0, errors.New("schematest invariant: string objective seed digest is too short")
	}

	return binary.BigEndian.Uint64(digest[:stringSeedBytes]), nil
}

func validateStringSeedObjective(objective stringObjective) error {
	if objective.owner.node == nil || objective.owner.node.schemaShape == nil || objective.owner.source == nil {
		return errors.New("schematest invariant: string objective seed has no owner schema")
	}

	if objective.owner.occurrence.usePointer == "" {
		return errors.New("schematest invariant: string objective seed has no owner pointer")
	}

	if objective.rule == "" || objective.level == "" {
		return errors.New("schematest invariant: string objective seed has no rule or level")
	}

	return nil
}

func writeStringSeedPrefix(hasher hash.Hash, objective stringObjective) error {
	return writeCanonicalStrings(
		hasher,
		"schematest-v1\x00",
		objective.owner.occurrence.usePointer,
		"\x00",
	)
}

func writeStringSeedSuffix(hasher hash.Hash, objective stringObjective) error {
	return writeCanonicalStrings(hasher, "\x00", objective.rule, "\x00", objective.level)
}

func writeCanonicalStrings(writer hash.Hash, values ...string) error {
	for _, value := range values {
		if err := writeCanonicalString(writer, value); err != nil {
			return err
		}
	}

	return nil
}

func stringSearchSeedForInterval(seed uint64, interval stringUnitInterval) uint64 {
	seed ^= uint64(interval.low)<<stringSeedIntervalShift | uint64(interval.high)
	seed ^= seed >> stringSeedFirstMixShift
	seed *= stringSeedMixMultiplier
	seed ^= seed >> stringSeedSecondMixShift

	return seed
}
