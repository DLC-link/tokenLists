package chains

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/migratooor/tokenLists/generators/common/models"
)

var OPTIMISM = TChain{
	ID:            10,
	RpcURI:        `https://mainnet.optimism.io`,
	MaxBlockRange: 100_000_000,
	MaxBatchSize:  math.MaxInt64,
	MulticallContract: TContractData{
		Address: common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
		Block:   0,
	},
	Coin: models.TokenListToken{
		Address:  `0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE`,
		Name:     `Ethereum`,
		Symbol:   `ETH`,
		LogoURI:  `https://assets.smold.app/api/token/10/0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE/logo-128.png`,
		ChainID:  1,
		Decimals: 18,
	},
	IgnoredTokens: []common.Address{},
}
