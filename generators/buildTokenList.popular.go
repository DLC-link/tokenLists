package main

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/migratooor/tokenLists/generators/common/chains"
	"github.com/migratooor/tokenLists/generators/common/helpers"
	"github.com/migratooor/tokenLists/generators/common/models"
)

func buildPopularList() {
	tokenList := helpers.LoadTokenListFromJsonFile(`popular.json`)
	tokenList.Name = `Popular tokens`
	tokenList.LogoURI = `https://raw.githubusercontent.com/smoldapp/tokenLists/main/.github/tokenlistooor.svg`
	tokenList.Description = `A curated list of popular tokens from all the token lists on tokenlistooor.`

	/**************************************************************************
	** Create a map of all tokens from all lists and only add the missing ones
	** in it. Map are WAY faster than arrays fir our use case
	**************************************************************************/
	allTokens := make(map[uint64]map[string]models.TokenListToken)
	allTokensPlain := []models.TokenListToken{}
	listsPerChain := make(map[uint64][]string)

	for _, chain := range chains.CHAINS {
		allTokensPlain = append(allTokensPlain, chain.Coin)
	}

	/**************************************************************************
	** We want to know which tokens to add to the aggregated tokenlistooor list
	** and to do that we need to know in how many lists they are present.
	** This is chain sensitive: we need a token to be available in at least
	** 50% of the lists for a given chain to be added to the aggregated list.
	**************************************************************************/
	for name, generatorData := range GENERATORS {
		if name == `tokenlistooor` {
			continue
		}
		if generatorData.GeneratorType == GeneratorPool {
			continue
		}

		initialCount := 1
		tokenList := helpers.LoadTokenListFromJsonFile(name + `.json`)
		for _, token := range tokenList.Tokens {
			if !chains.IsChainIDSupported(token.ChainID) {
				continue
			}
			if _, ok := listsPerChain[token.ChainID]; !ok {
				listsPerChain[token.ChainID] = []string{}
			}

			if !helpers.Includes(listsPerChain[token.ChainID], name) {
				listsPerChain[token.ChainID] = append(listsPerChain[token.ChainID], name)
			}

			if _, ok := allTokens[token.ChainID]; !ok {
				allTokens[token.ChainID] = make(map[string]models.TokenListToken)
			}

			if existingToken, ok := allTokens[token.ChainID][helpers.ToAddress(token.Address)]; ok {
				allTokens[token.ChainID][helpers.ToAddress(token.Address)] = models.TokenListToken{
					Address:    existingToken.Address,
					Name:       helpers.SafeString(existingToken.Name, token.Name),
					Symbol:     helpers.SafeString(existingToken.Symbol, token.Symbol),
					LogoURI:    helpers.SafeString(existingToken.LogoURI, token.LogoURI),
					Decimals:   helpers.SafeInt(existingToken.Decimals, token.Decimals),
					ChainID:    token.ChainID,
					Occurrence: existingToken.Occurrence + 1,
				}
			} else {
				tokenInitialOccurence := initialCount
				for _, extraToken := range chains.CHAINS[token.ChainID].ExtraTokens {
					if common.HexToAddress(token.Address) == extraToken {
						tokenInitialOccurence = 10
					}
				}

				allTokens[token.ChainID][helpers.ToAddress(token.Address)] = models.TokenListToken{
					Address:    helpers.ToAddress(token.Address),
					Name:       helpers.SafeString(token.Name, ``),
					Symbol:     helpers.SafeString(token.Symbol, ``),
					LogoURI:    helpers.SafeString(token.LogoURI, ``),
					Decimals:   helpers.SafeInt(token.Decimals, 18),
					ChainID:    token.ChainID,
					Occurrence: tokenInitialOccurence,
				}
			}
		}
	}

	for chainID, tokens := range allTokens {
		for _, token := range tokens {
			if _, ok := listsPerChain[chainID]; !ok {
				continue
			}
			chainCount := len(listsPerChain[uint64(chainID)])
			if token.Occurrence >= int(math.Ceil(float64(chainCount)*0.5)) {
				allTokensPlain = append(allTokensPlain, token)
			}
		}
	}

	tokens := helpers.GetTokensFromList(allTokensPlain)
	for _, token := range allTokensPlain {
		for i, t := range tokens {
			if common.HexToAddress(token.Address).Hex() == common.HexToAddress(t.Address).Hex() {
				tokens[i].Occurrence = token.Occurrence
			}
		}
	}

	helpers.SaveTokenListInJsonFile(tokenList, tokens, `popular.json`, helpers.SavingMethodStandard)
}
