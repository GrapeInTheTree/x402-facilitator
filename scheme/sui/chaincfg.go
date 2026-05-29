package sui

import (
	"maps"
	"slices"
)

type NetworkInfo struct {
	Network         string
	NetworkName     string
	NetworkID       string
	DefaultURLs     []string
	StablecoinTypes map[string]string
}

func GetNetworkInfo(network string) *NetworkInfo {
	info, ok := networkInfo[network]
	if !ok {
		return nil
	}
	info.DefaultURLs = slices.Clone(info.DefaultURLs)
	info.StablecoinTypes = maps.Clone(info.StablecoinTypes)
	return &info
}

func GetNetworkName(network string) string {
	info := GetNetworkInfo(network)
	if info == nil {
		return ""
	}
	return info.NetworkName
}

func GetNetworkID(network string) string {
	info := GetNetworkInfo(network)
	if info == nil {
		return ""
	}
	return info.NetworkID
}

func GetDefaultURLs(network string) []string {
	info := GetNetworkInfo(network)
	if info == nil {
		return nil
	}
	return slices.Clone(info.DefaultURLs)
}

func GetGaslessStablecoinTypes(network string) []string {
	info := GetNetworkInfo(network)
	if info == nil {
		return nil
	}

	types := make([]string, 0, len(info.StablecoinTypes))
	seen := make(map[string]struct{}, len(info.StablecoinTypes))
	for _, coinType := range DefaultGaslessStablecoinTypeList {
		if _, ok := stablecoinTypeInMap(info.StablecoinTypes, coinType); ok {
			types = append(types, coinType)
			seen[NormalizeType(coinType)] = struct{}{}
		}
	}
	for _, coinType := range info.StablecoinTypes {
		if _, ok := seen[NormalizeType(coinType)]; !ok {
			types = append(types, coinType)
		}
	}
	return types
}

func GetGaslessStablecoinType(network, asset string) (string, bool) {
	info := GetNetworkInfo(network)
	if info == nil {
		return "", false
	}
	for symbol, coinType := range info.StablecoinTypes {
		if NormalizeType(symbol) == NormalizeType(asset) {
			return coinType, true
		}
	}
	normalizedAsset := NormalizeType(asset)
	for _, coinType := range info.StablecoinTypes {
		if NormalizeType(coinType) == normalizedAsset {
			return coinType, true
		}
	}
	return "", false
}

func stablecoinTypeInMap(stablecoinTypes map[string]string, coinType string) (string, bool) {
	normalizedCoinType := NormalizeType(coinType)
	for symbol, candidate := range stablecoinTypes {
		if NormalizeType(candidate) == normalizedCoinType {
			return symbol, true
		}
	}
	return "", false
}

var networkInfo = map[string]NetworkInfo{
	"sui:mainnet": {
		Network:         "sui:mainnet",
		NetworkName:     "Sui Mainnet",
		NetworkID:       "mainnet",
		DefaultURLs:     []string{"https://sui-rpc.publicnode.com", "https://fullnode.mainnet.sui.io:443"},
		StablecoinTypes: defaultStablecoinTypesBySymbol(),
	},
	"sui:testnet": {
		Network:         "sui:testnet",
		NetworkName:     "Sui Testnet",
		NetworkID:       "testnet",
		DefaultURLs:     []string{"https://sui-testnet-rpc.publicnode.com", "https://fullnode.testnet.sui.io:443"},
		StablecoinTypes: defaultStablecoinTypesBySymbol(),
	},
	"sui:localnet": {
		Network:         "sui:localnet",
		NetworkName:     "Sui Localnet",
		NetworkID:       "localnet",
		DefaultURLs:     []string{"http://127.0.0.1:9000"},
		StablecoinTypes: defaultStablecoinTypesBySymbol(),
	},
}

func defaultStablecoinTypesBySymbol() map[string]string {
	return map[string]string{
		"USDC":     USDCType,
		"USDSUI":   USDSUIType,
		"SUI_USDE": SUIUSDEType,
		"USDY":     USDYType,
		"FDUSD":    FDUSDType,
		"AUSD":     AUSDType,
		"USDB":     USDBType,
	}
}
