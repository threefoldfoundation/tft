package state

import (
	"encoding/json"
	"os"
)

type Blockheight struct {
	LastHeight uint64 `json:"lastHeight"`
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
	updatedPersistency, err := json.Marshal(Blockheight{LastHeight: height})
	if err != nil {
		return err
	}

	return os.WriteFile(b.location, updatedPersistency, 0644)
}

func (b *ChainPersistency) GetHeight() (height uint64, err error) {
	var blockheight Blockheight
	file, err := os.ReadFile(b.location)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return
	}
	err = json.Unmarshal(file, &blockheight)
	if err != nil {
		return
	}

	return blockheight.LastHeight, nil
}
