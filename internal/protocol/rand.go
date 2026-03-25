package protocol

import "math/rand/v2"

var rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

// GenerateSwapMultipleValue generates a random encryption multiple in range [6, 12].
// Must match the range used by the EO client (eolib generates values in this range).
func GenerateSwapMultipleValue() int {
	return rng.IntN(7) + 6
}
