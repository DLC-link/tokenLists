package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/migratooor/tokenLists/generators/common/helpers"
	"github.com/migratooor/tokenLists/generators/common/logs"
)

func fetchUniswapTokenList() TokenListData {
	resp, err := http.Get(`https://tokens.uniswap.org`)
	if err != nil {
		logs.Error(err)
		return TokenListData{}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logs.Error(err)
		return TokenListData{}
	}

	var list TokenListData
	if err := json.Unmarshal(body, &list); err != nil {
		logs.Error(err)
		return TokenListData{}
	}
	return list
}

func buildUniswapTokenList() {
	tokenList := loadTokenListFromJsonFile(`uniswap.json`)
	originalTokenList := fetchUniswapTokenList()
	tokenList.Name = originalTokenList.Name
	tokenList.LogoURI = originalTokenList.LogoURI
	tokenList.Keywords = originalTokenList.Keywords

	for _, token := range originalTokenList.Tokens {
		if helpers.IsIgnoredChain(uint64(token.ChainID)) {
			continue
		}
		if (token.Name == `` || token.Symbol == `` || token.Decimals == 0) || helpers.IsIgnoredToken(uint64(token.ChainID), common.HexToAddress(token.Address)) {
			continue
		}

		key := GetKey(uint64(token.ChainID), common.HexToAddress(token.Address))
		tokenList.NextTokensMap[key] = token
	}

	saveTokenListInJsonFile(tokenList, tokenList.Tokens, `uniswap.json`, Standard)
}