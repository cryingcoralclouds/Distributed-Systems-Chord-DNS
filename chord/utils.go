package chord

import (
	"crypto/md5"
	"math/big"
	"math/rand"
)

func HashKey(input string) *big.Int {
	// Generate a seed using MD5 hash of the input string for deterministic random behaviour
	seed := md5.Sum([]byte(input))

	// Create a new random source with the seed
	r := rand.New(rand.NewSource(int64(seed[0]) | int64(seed[1])<<8 | int64(seed[2])<<16 | int64(seed[3])<<24))

	maxInt := big.NewInt(1024) // Key space of 2^10

	// Generate a pseudo-random number in the reduced key space
	randomBigInt := new(big.Int).Rand(r, maxInt)

	return randomBigInt
}

// A utility function that checks if a given ID falls between two other IDs in a circular manner, accounting for the ring topology of Chord.
func Between(id, start, end *big.Int, inclusive bool) bool {
	if id == nil || start == nil || end == nil {
		// log.Printf("[Between] Nil value detected in comparison")
		return false
	}

	// Convert to mod ring size to ensure proper comparison
	id = new(big.Int).Mod(id, RingSize)
	start = new(big.Int).Mod(start, RingSize)
	end = new(big.Int).Mod(end, RingSize)

	// log.Printf("[Between] Checking if %s is between %s and %s (inclusive: %v)",
	//     id.String(), start.String(), end.String(), inclusive)

	// If start equals end
	if start.Cmp(end) == 0 {
		result := inclusive
		// log.Printf("[Between] Start equals end, returning %v", result)
		return result
	}

	// If end is less than start (wrapping around the ring)
	if start.Cmp(end) > 0 {
		// id should be greater than start OR less than end
		result := id.Cmp(start) > 0 || id.Cmp(end) < 0
		// log.Printf("[Between] Ring wrap case, returning %v", result)
		return result
	}

	// Normal case
	if inclusive {
		result := id.Cmp(start) >= 0 && id.Cmp(end) <= 0
		// log.Printf("[Between] Normal case (inclusive), returning %v", result)
		return result
	}
	result := id.Cmp(start) > 0 && id.Cmp(end) < 0
	// log.Printf("[Between] Normal case (exclusive), returning %v", result)
	return result
}

func CompareNodes(n1, n2 *big.Int) string {
	if n1.Cmp(n2) < 0 {
		return "less than"
	} else if n1.Cmp(n2) > 0 {
		return "greater than"
	}
	return "equal to"
}
