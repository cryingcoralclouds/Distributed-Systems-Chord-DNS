package chord

import (
    "crypto/sha1"
    "math/big"
)

// Takes a string input and returns its SHA-1 hash as a big integer. This is used for generating unique identifiers for keys and nodes.
func hashKey(input string) *big.Int {
	hash := sha1.New()
	hash.Write([]byte(input))
	return new(big.Int).SetBytes(hash.Sum(nil))
}

// A utility function that checks if a given ID falls between two other IDs in a circular manner, accounting for the ring topology of Chord.
func between(id, start, end *big.Int, inclusive bool) bool {
	if id == nil || start == nil || end == nil {
		return false
	}

	if start.Cmp(end) == 0 {
		return inclusive
	}

	if start.Cmp(end) < 0 {
		if inclusive {
			return id.Cmp(start) >= 0 && id.Cmp(end) <= 0
		}
		return id.Cmp(start) > 0 && id.Cmp(end) < 0
	}

	if inclusive {
		return id.Cmp(start) >= 0 || id.Cmp(end) <= 0
	}
	return id.Cmp(start) > 0 || id.Cmp(end) < 0
}