package bridge

// Bridge implements the actual briding
type Bridge struct {
	Persistency *ChainPersistency
	config      *BridgeConfig
	synced      bool
}

// NewBridge creates a new Bridge.
func NewBridge(config *BridgeConfig) (br *Bridge, err error) {

	blockPersistency := newChainPersistency(config.PersistencyFile)

	br = &Bridge{
		Persistency: blockPersistency,
		config:      config,
	}

	return
}

func (br *Bridge) Start() (err error) {
	return
}
