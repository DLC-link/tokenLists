package main

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/migratooor/tokenLists/generators/common/ethereum"
	"github.com/migratooor/tokenLists/generators/common/helpers"
	"github.com/migratooor/tokenLists/generators/common/logs"
)

type JSONSaveTokensMethods string

const (
	Standard JSONSaveTokensMethods = "Standard"
	Append   JSONSaveTokensMethods = "Append"
)

// loadTokenListFromJsonFile loads a token list from a json file.
func loadTokenListFromJsonFile(filePath string) TokenListData[TokenListToken] {
	var tokenList TokenListData[TokenListToken]
	content, err := os.ReadFile(helpers.BASE_PATH + `/lists/` + filePath)
	if err != nil {
		logs.Error(err)
		if errors.Is(err, os.ErrNotExist) {
			os.WriteFile(helpers.BASE_PATH+`/lists/`+filePath, []byte(`{}`), 0644)
		}
		return InitTokenList()
	}
	if err = json.Unmarshal(content, &tokenList); err != nil {
		logs.Error(err)
		return InitTokenList()
	}

	tokenList.PreviousTokensMap = make(map[string]TokenListToken)
	for _, token := range tokenList.Tokens {
		if !helpers.IsChainIDSupported(token.ChainID) {
			continue
		}
		key := getKey(token.ChainID, common.HexToAddress(token.Address))
		tokenList.PreviousTokensMap[key] = token
	}
	tokenList.NextTokensMap = make(map[string]TokenListToken)
	return tokenList
}

// saveTokenListInJsonFile saves a token list in a json file
func saveTokenListInJsonFile(
	tokenList TokenListData[TokenListToken],
	tokensMaybeDuplicates []TokenListToken,
	filePath string,
	method JSONSaveTokensMethods,
) error {
	tokens := []TokenListToken{}
	addresses := make(map[string]bool)
	for _, token := range tokensMaybeDuplicates {
		key := token.Address + strconv.FormatUint(token.ChainID, 10)
		if _, ok := addresses[key]; !ok {
			addresses[key] = true
			tokens = append(tokens, token)
		}
	}

	/**************************************************************************
	** First part is transforming the token list into a map. This will allow
	** us to detect the changes in the token list and to directly access a
	** token by its address.
	** If the method is set to "Append", we will first need to load the
	** previous token list.
	**************************************************************************/
	if method == Append {
		for _, token := range tokenList.PreviousTokensMap {
			if !helpers.IsChainIDSupported(token.ChainID) {
				continue
			}
			if (token.Name == `` || token.Symbol == `` || token.Decimals == 0) || helpers.IsIgnoredToken(token.ChainID, common.HexToAddress(token.Address)) {
				continue
			}
			newToken, err := SetToken(
				common.HexToAddress(token.Address),
				token.Name,
				token.Symbol,
				token.LogoURI,
				token.ChainID,
				token.Decimals,
			)
			if err != nil {
				continue
			}
			tokenList.NextTokensMap[getKey(token.ChainID, common.HexToAddress(token.Address))] = newToken
		}
	}

	for _, token := range tokens {
		if !helpers.IsChainIDSupported(token.ChainID) {
			continue
		}
		newToken, err := SetToken(
			common.HexToAddress(token.Address),
			token.Name,
			token.Symbol,
			token.LogoURI,
			token.ChainID,
			token.Decimals,
		)
		if err != nil {
			logs.Error(err)
			continue
		}
		tokenList.NextTokensMap[getKey(token.ChainID, common.HexToAddress(token.Address))] = newToken
	}

	if len(tokenList.NextTokensMap) == 0 {
		return errors.New(`token list is empty`)
	}
	tokenList.Timestamp = time.Now().UTC().Format(`02/01/2006 15:04:05`)
	tokenList.Tokens = []TokenListToken{}

	/**************************************************************************
	** Detect the changes in the token list.
	** If a token is removed, the major version is bumped.
	** If a token is added, the minor version is bumped.
	** If a token is modified, the patch version is bumped.
	** Skip if we are not using the standard method.
	**************************************************************************/
	shouldBumpMajor := false
	shouldBumpMinor := false
	shouldBumpPatch := false
	for _, token := range tokenList.NextTokensMap {
		key := getKey(token.ChainID, common.HexToAddress(token.Address))
		if _, ok := tokenList.PreviousTokensMap[key]; !ok {
			shouldBumpMinor = true
		} else if !reflect.DeepEqual(token, tokenList.PreviousTokensMap[key]) {
			shouldBumpPatch = true
		}
		tokenList.Tokens = append(tokenList.Tokens, token)
		delete(tokenList.PreviousTokensMap, key)
	}
	if len(tokenList.PreviousTokensMap) > 0 {
		shouldBumpMajor = true
	}

	/**************************************************************************
	** If there are no changes, we will just return.
	**************************************************************************/
	if !shouldBumpMajor && !shouldBumpMinor && !shouldBumpPatch {
		return nil
	}

	if shouldBumpMajor {
		tokenList.Version.Major++
		tokenList.Version.Minor = 0
		tokenList.Version.Patch = 0
	} else if shouldBumpMinor {
		tokenList.Version.Minor++
		tokenList.Version.Patch = 0
	} else if shouldBumpPatch {
		tokenList.Version.Patch++
	}

	/**************************************************************************
	** To make it easy to work with the list, we will sort the token by their
	** chainID in ascending order.
	**************************************************************************/
	tokeListPerChainID := make(map[uint64][]TokenListToken)
	keys := make([]string, 0, len(tokenList.NextTokensMap))
	for k := range tokenList.NextTokensMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		chainID, _ := strconv.ParseUint(strings.Split(k, `_`)[0], 10, 64)
		if _, ok := tokeListPerChainID[chainID]; !ok {
			tokeListPerChainID[chainID] = []TokenListToken{}
		}

		token := tokenList.NextTokensMap[k]
		if (token.Name == `` || token.Symbol == `` || token.Decimals == 0) || helpers.IsIgnoredToken(token.ChainID, common.HexToAddress(token.Address)) {
			continue
		}
		tokenList.Tokens[i] = tokenList.NextTokensMap[k]
		tokeListPerChainID[chainID] = append(tokeListPerChainID[chainID], tokenList.NextTokensMap[k])
	}

	/**************************************************************************
	** Then we will just save the unified token list in a json file as well as
	** each individual token list per chainID.
	**************************************************************************/
	jsonData, err := json.MarshalIndent(tokenList, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(helpers.BASE_PATH+`/lists/`+filePath, jsonData, 0644); err != nil {
		return err
	}

	for chainID, tokens := range tokeListPerChainID {
		if !helpers.IsChainIDSupported(chainID) {
			continue
		}
		chainIDStr := strconv.FormatUint(chainID, 10)
		tokenList.Tokens = tokens

		jsonData, err := json.MarshalIndent(tokenList, "", "  ")
		if err != nil {
			logs.Error(err)
			return err
		}
		if err := helpers.CreateFile(helpers.BASE_PATH + `/lists/` + chainIDStr); err != nil {
			logs.Error(err)
			return err
		}

		if err = os.WriteFile(helpers.BASE_PATH+`/lists/`+chainIDStr+`/`+filePath, jsonData, 0644); err != nil {
			logs.Error(err)
			return err
		}
	}

	return nil
}

// getKey returns the key of a token in a specific format to make it sortable
func getKey(chainID uint64, address common.Address) string {
	chainIDStr := strconv.FormatUint(chainID, 10)
	chainIDStr = strings.Repeat("0", 18-len(chainIDStr)) + chainIDStr
	return chainIDStr + `_` + address.Hex()
}

func initSyncMap[T any](chainIDs map[uint64]T) *sync.Map {
	tokensForChainIDSyncMap := sync.Map{}
	for chainID := range chainIDs {
		tokensForChainIDSyncMap.Store(chainID, []TokenListToken{})
	}
	return &tokensForChainIDSyncMap
}

func extractSyncMap(mapper *sync.Map) []TokenListToken {
	tokenList := []TokenListToken{}
	mapper.Range(func(chainID, syncMapRaw interface{}) bool {
		syncMap, _ := syncMapRaw.([]TokenListToken)
		tokenList = append(tokenList, syncMap...)
		return true
	})
	return tokenList
}

func loadAllTokens() map[uint64]map[string]TokenListToken {
	allTokens := make(map[uint64]map[string]TokenListToken)
	for name := range GENERATORS {
		tokenList := loadTokenListFromJsonFile(name + `.json`)
		for _, token := range tokenList.Tokens {
			if _, ok := allTokens[token.ChainID]; !ok {
				allTokens[token.ChainID] = make(map[string]TokenListToken)
			}
			if existingToken, ok := allTokens[token.ChainID][helpers.ToAddress(token.Address)]; ok {
				allTokens[token.ChainID][helpers.ToAddress(token.Address)] = TokenListToken{
					Address:    existingToken.Address,
					Name:       helpers.SafeString(existingToken.Name, token.Name),
					Symbol:     helpers.SafeString(existingToken.Symbol, token.Symbol),
					LogoURI:    helpers.SafeString(existingToken.LogoURI, token.LogoURI),
					Decimals:   helpers.SafeInt(existingToken.Decimals, token.Decimals),
					ChainID:    token.ChainID,
					Occurrence: existingToken.Occurrence + 1,
				}
			} else {
				allTokens[token.ChainID][helpers.ToAddress(token.Address)] = TokenListToken{
					Address:    helpers.ToAddress(token.Address),
					Name:       helpers.SafeString(token.Name, ``),
					Symbol:     helpers.SafeString(token.Symbol, ``),
					LogoURI:    helpers.SafeString(token.LogoURI, ``),
					Decimals:   helpers.SafeInt(token.Decimals, 18),
					ChainID:    token.ChainID,
					Occurrence: 1,
				}
			}
		}
	}
	return allTokens
}

func loadAllTokenLogoURI() map[uint64]map[string]string {
	allTokenLogoURI := make(map[uint64]map[string]string)
	for name := range GENERATORS {
		tokenList := loadTokenListFromJsonFile(name + `.json`)
		for _, token := range tokenList.Tokens {
			if _, ok := allTokenLogoURI[token.ChainID]; !ok {
				allTokenLogoURI[token.ChainID] = make(map[string]string)
			}
			currentIcon := allTokenLogoURI[token.ChainID][helpers.ToAddress(token.Address)]
			if currentIcon == helpers.DEFAULT_SMOL_NOT_FOUND ||
				currentIcon == helpers.DEFAULT_PARASWAP_NOT_FOUND ||
				currentIcon == helpers.DEFAULT_ETHERSCAN_NOT_FOUND ||
				currentIcon == `` {
				baseIcon := helpers.UseIcon(token.ChainID, token.Name+` - `+token.Symbol, common.HexToAddress(token.Address), token.LogoURI)
				allTokenLogoURI[token.ChainID][helpers.ToAddress(token.Address)] = baseIcon
			}

		}
	}
	helpers.ExistingTokenLogoURI = allTokenLogoURI
	return allTokenLogoURI
}

func retrieveBasicInformations(chainID uint64, addresses []common.Address) map[string]*ethereum.TERC20 {
	erc20Map := make(map[string]*ethereum.TERC20)
	missingAddresses := []common.Address{}

	if !helpers.IsChainIDSupported(chainID) {
		return erc20Map
	}

	for _, v := range addresses {
		if token, ok := ALL_EXISTING_TOKENS[chainID][v.Hex()]; ok {
			if token.Name == `` && token.Symbol == `` && token.Decimals == 0 {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing name, symbol and decimals for token:`, token.Address, `on chain:`, chainID)
			} else if token.Name == `` && token.Symbol == `` {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing name and symbol for token:`, token.Address, `on chain:`, chainID)
			} else if token.Name == `` && token.Decimals == 0 {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing name and decimals for token:`, token.Address, `on chain:`, chainID)
			} else if token.Symbol == `` && token.Decimals == 0 {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing symbol and decimals for token:`, token.Address, `on chain:`, chainID)
			} else if token.Name == `` {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing name for token:`, token.Address, `on chain:`, chainID)
			} else if token.Symbol == `` {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing symbol for token:`, token.Address, `on chain:`, chainID)
			} else if token.Decimals == 0 {
				logs.Warning(`[ALL_EXISTING_TOKENS]: Missing decimals for token:`, token.Address, `on chain:`, chainID)
			}
			erc20Map[v.Hex()] = &ethereum.TERC20{
				Address:  v,
				Name:     token.Name,
				Symbol:   token.Symbol,
				Decimals: uint64(token.Decimals),
				ChainID:  chainID,
			}
		} else {
			missingAddresses = append(missingAddresses, v)
		}
	}
	erc20FromChain := ethereum.FetchBasicInformations(chainID, missingAddresses)
	for k, v := range erc20FromChain {
		erc20Map[k] = v
		if _, ok := ALL_EXISTING_TOKENS[chainID]; !ok {
			ALL_EXISTING_TOKENS[chainID] = make(map[string]TokenListToken)
		}
		if v.Name == `` && v.Symbol == `` {
			logs.Warning(`[FETCHED_TOKEN] - Missing name and symbol for token:`, v.Address, `on chain:`, chainID)
		} else if v.Name == `` {
			logs.Warning(`[FETCHED_TOKEN] - Missing name for token:`, v.Address, `on chain:`, chainID)
		} else if v.Symbol == `` {
			logs.Warning(`[FETCHED_TOKEN] - Missing symbol for token:`, v.Address, `on chain:`, chainID)
		}
		ALL_EXISTING_TOKENS[chainID][v.Address.Hex()] = TokenListToken{
			Address:    v.Address.Hex(),
			Name:       v.Name,
			Symbol:     v.Symbol,
			LogoURI:    ``,
			Decimals:   int(v.Decimals),
			ChainID:    chainID,
			Occurrence: 1,
		}
	}
	return erc20Map
}

func groupByChainID(tokens []TokenListToken) map[uint64][]common.Address {
	tokensPerChainID := make(map[uint64][]common.Address)
	for _, token := range tokens {
		if !helpers.IsChainIDSupported(token.ChainID) {
			continue
		}
		tokensPerChainID[token.ChainID] = append(tokensPerChainID[token.ChainID], common.HexToAddress(token.Address))
	}
	return tokensPerChainID
}

func getExistingLogo(chainID uint64, lookingFor common.Address, slice []TokenListToken) string {
	for _, token := range slice {
		if !helpers.IsChainIDSupported(token.ChainID) {
			continue
		}
		if token.Address == lookingFor.Hex() && chainID == token.ChainID {
			return token.LogoURI
		}
	}
	return ``
}

func SetToken(
	address common.Address,
	name string, symbol string, logoURI string,
	chainID uint64, decimals int,
) (TokenListToken, error) {
	token := TokenListToken{}
	if name == `` {
		return token, errors.New(`token name is empty`)
	}
	if symbol == `` {
		return token, errors.New(`token symbol is empty`)
	}
	if decimals == 0 {
		return token, errors.New(`token decimals is 0`)
	}
	if helpers.IsIgnoredToken(chainID, address) {
		return token, errors.New(`token is ignored`)
	}
	if chainID == 0 || !helpers.IsChainIDSupported(chainID) {
		return token, errors.New(`chainID is ignored`)
	}
	if address.Hex() == common.HexToAddress(`0x2791bca1f2de4661ed88a30c99a7a9449aa84174`).Hex() && chainID == 137 {
		name = `Bridged USD Coin (PoS)`
		symbol = `USDC.e`
	}

	token.ChainID = chainID
	token.Decimals = decimals
	token.Address = address.Hex()
	token.Name = name
	token.Symbol = symbol
	token.LogoURI = helpers.UseIcon(chainID, token.Name+` - `+token.Symbol, address, logoURI)
	return token, nil
}

func fetchTokenList(tokensFromList []TokenListToken) []TokenListToken {
	tokens := []TokenListToken{}
	grouped := groupByChainID(tokensFromList)

	for chainID, tokensForChain := range grouped {
		if !helpers.IsChainIDSupported(chainID) {
			continue
		}

		tokensInfo := retrieveBasicInformations(chainID, tokensForChain)
		for _, existingToken := range tokensForChain {
			if token, ok := tokensInfo[existingToken.Hex()]; ok {
				if newToken, err := SetToken(
					token.Address,
					token.Name,
					token.Symbol,
					getExistingLogo(chainID, existingToken, tokensFromList),
					chainID,
					int(token.Decimals),
				); err == nil {
					tokens = append(tokens, newToken)
				}
			}
		}
	}

	return tokens
}
