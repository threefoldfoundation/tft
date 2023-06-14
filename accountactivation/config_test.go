package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStellarConfigValidate(t *testing.T) {
	c := Config{}
	assert.Error(t, c.Validate())
	c.StellarNetwork = "production"
	c.StellarSecret = "SBVM45L3DA4QA4GRGOZVOKEMRI6LGJXBGOFGHUTCWL3LW6H7KSHCYUTS"
	assert.NoError(t, c.Validate())
}
