package ethereum

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/migratooor/tokenLists/generators/common/helpers"
	"github.com/migratooor/tokenLists/generators/common/logs"
)

// RPC contains the ethclient.Client for a specific chainID
var RPC = map[uint64]*ethclient.Client{}

// MulticallClientForChainID holds the multicall client for a specific chainID
var MulticallClientForChainID = make(map[uint64]TEthMultiCaller)

// RPC_ENDPOINTS contains the node endpoints to connect the blockchains
var RPC_ENDPOINTS = map[uint64]string{
	1:     `https://eth.public-rpc.com`,
	5:     `https://goerli.optimism.io`,
	10:    `https://mainnet.optimism.io`,
	56:    `https://1rpc.io/bnb`,
	100:   `https://rpc.gnosis.gateway.fm`,
	137:   `https://polygon.llamarpc.com`,
	250:   `https://rpc.ftm.tools`,
	324:   `https://mainnet.era.zksync.io`,
	1101:  `https://zkevm-rpc.com`,
	8453:  `https://mainnet.base.org/`,
	42161: `https://arbitrum.public-rpc.com`,
	43114: `https://1rpc.io/avax/c`,
}

// DEFAULT_RPC_ENDPOINTS contains the node endpoints to connect the blockchains
var DEFAULT_RPC_ENDPOINTS = map[uint64]string{
	1:     `https://eth.public-rpc.com`,
	5:     `https://goerli.optimism.io`,
	10:    `https://mainnet.optimism.io`,
	56:    `https://1rpc.io/bnb`,
	100:   `https://rpc.gnosis.gateway.fm`,
	137:   `https://polygon.llamarpc.com`,
	250:   `https://rpc.ftm.tools`,
	324:   `https://mainnet.era.zksync.io`,
	1101:  `https://zkevm-rpc.com`,
	8453:  `https://mainnet.base.org/`,
	42161: `https://arbitrum.public-rpc.com`,
	43114: `https://1rpc.io/avax/c`,
}

// MULTICALL_ADDRESSES contains the address of the multicall2 contract for a specific chainID
var MULTICALL_ADDRESSES = map[uint64]common.Address{
	1:     common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	5:     common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	10:    common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	56:    common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	100:   common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	137:   common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	250:   common.HexToAddress(`0x470ADB45f5a9ac3550bcFFaD9D990Bf7e2e941c9`),
	324:   common.HexToAddress(`0xF9cda624FBC7e059355ce98a31693d299FACd963`),
	1101:  common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	42161: common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	8453:  common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
	43114: common.HexToAddress(`0xca11bde05977b3631167028862be2a173976ca11`),
}

func Init() {
	// Load the RPC_ENDPOINTS from the env variables
	for chainID := range helpers.SUPPORTED_CHAIN_IDS {
		RPC_ENDPOINTS[chainID] = helpers.UseEnv(`RPC_URI_FOR_`+strconv.FormatUint(chainID, 10), RPC_ENDPOINTS[chainID])
	}

	// Init the RPC clients
	for chainID := range helpers.SUPPORTED_CHAIN_IDS {
		client, err := ethclient.Dial(GetRPCURI(chainID))
		if err != nil {
			os.Setenv(`RPC_URI_FOR_`+strconv.FormatUint(chainID, 10), DEFAULT_RPC_ENDPOINTS[chainID])
			RPC_ENDPOINTS[chainID] = helpers.UseEnv(`RPC_URI_FOR_`+strconv.FormatUint(chainID, 10), RPC_ENDPOINTS[chainID])
			client, err = ethclient.Dial(RPC_ENDPOINTS[chainID])
			if err != nil {
				logs.Error(err)
				continue
			}
		}
		RPC[chainID] = client
	}

	// Create the multicall client for all the chains supported by yDaemon
	for chainID := range helpers.SUPPORTED_CHAIN_IDS {
		MulticallClientForChainID[chainID] = NewMulticall(GetRPCURI(chainID), MULTICALL_ADDRESSES[chainID])
	}
}

// GetRPC returns the current connection for a specific chain
func GetRPC(chainID uint64) *ethclient.Client {
	return RPC[chainID]
}

// GetRPCURI returns the URI to use to connect to the node for a specific chainID
func GetRPCURI(chainID uint64) string {
	return RPC_ENDPOINTS[chainID]
}

func randomSigner() *bind.TransactOpts {
	privateKey, _ := crypto.GenerateKey()
	signer, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1))
	signer.NoSend = true
	signer.Context = context.Background()
	signer.GasLimit = math.MaxUint64
	signer.GasFeeCap = big.NewInt(0)
	signer.GasTipCap = big.NewInt(0)
	signer.GasPrice = big.NewInt(0)
	return signer
}

// decodeString decodes a string from a slice of interfaces
func decodeString(something []interface{}, fallback string) string {
	if len(something) == 0 {
		return fallback
	}
	return something[0].(string)
}

// decodeHex decodes a hax from a slice of interfaces and try to convert it to a string
func decodeHex(something []interface{}, fallback string) string {
	if len(something) == 0 {
		return fallback
	}
	asBytes32 := something[0].([32]uint8)
	if len(asBytes32) == 0 {
		return fallback
	}
	asBytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		if asBytes32[i] == 0 {
			asBytes = asBytes[:i]
			break
		}
		asBytes[i] = asBytes32[i]
	}
	asHex := hex.EncodeToString(asBytes[:])
	asString, err := hex.DecodeString(asHex)
	if err != nil {
		return fallback
	}
	return string(asString)
}

// decodeUint64 decodes a uint64 from a slice of interfaces
func decodeUint64(something []interface{}, fallback uint64) uint64 {
	if len(something) == 0 {
		return fallback
	}
	return uint64(something[0].(uint8))
}
