package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStellarConfigValidate(t *testing.T) {
	c := StellarConfig{}
	assert.Error(t, c.Validate())
	c.StellarNetwork = "production"
	c.StellarSeed = "SBVM45L3DA4QA4GRGOZVOKEMRI6LGJXBGOFGHUTCWL3LW6H7KSHCYUTS"
	c.StellarFeeWallet = "GBA4RKS7ELQ3B77INEHSHHDCIYJV7LNNPTUQVW5RL6DJJWDSIYRZFPF6"
	assert.NoError(t, c.Validate())
}
