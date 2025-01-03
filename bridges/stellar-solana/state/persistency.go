package state

import (
	"encoding/json"
	"os"
)

type Blockheight struct {
	LastHeight    uint64 `json:"lastHeight"`
	StellarCursor string `json:"stellarCursor"`
}

type ChainPersistency struct {
	location string
}

// NewChainPersistency creates new ChainPersistency object and returns a reference to it.
func NewChainPersistency(location string) *ChainPersistency {
	return &ChainPersistency{
		location: location,
	}
}

func (b *ChainPersistency) SaveHeight(height uint64) error {
	blockheight, err := b.GetHeight()
	if err != nil {
		return err
	}

	blockheight.LastHeight = height
	return b.Save(blockheight)
}

func (b *ChainPersistency) SaveStellarCursor(cursor string) error {
	blockheight, err := b.GetHeight()
	if err != nil {
		return err
	}

	blockheight.StellarCursor = cursor
	return b.Save(blockheight)
}

func (b *ChainPersistency) GetHeight() (*Blockheight, error) {
	var blockheight Blockheight
	file, err := os.ReadFile(b.location)
	if os.IsNotExist(err) {
		return &blockheight, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(file, &blockheight)
	if err != nil {
		return nil, err
	}

	return &blockheight, nil
}

func (b *ChainPersistency) Save(blockheight *Blockheight) error {
	updatedPersistency, err := json.Marshal(blockheight)
	if err != nil {
		return err
	}

	return os.WriteFile(b.location, updatedPersistency, 0o644)
}
