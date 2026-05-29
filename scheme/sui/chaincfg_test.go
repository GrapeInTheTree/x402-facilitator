package sui_test

import (
	"testing"

	"github.com/gosuda/x402-facilitator/scheme/sui"
	"github.com/stretchr/testify/require"
)

func TestGetNetworkInfoIncludesDefaultPublicNodeEndpoints(t *testing.T) {
	tests := []struct {
		network     string
		networkName string
		networkID   string
		defaultURL  string
	}{
		{"sui:mainnet", "Sui Mainnet", "mainnet", "https://sui-rpc.publicnode.com"},
		{"sui:testnet", "Sui Testnet", "testnet", "https://sui-testnet-rpc.publicnode.com"},
		{"sui:localnet", "Sui Localnet", "localnet", "http://127.0.0.1:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.network, func(t *testing.T) {
			info := sui.GetNetworkInfo(tt.network)
			require.NotNil(t, info)
			require.Equal(t, tt.network, info.Network)
			require.Equal(t, tt.networkName, info.NetworkName)
			require.Equal(t, tt.networkID, info.NetworkID)
			require.NotEmpty(t, info.DefaultURLs)
			require.Equal(t, tt.defaultURL, info.DefaultURLs[0])
			require.Equal(t, tt.networkName, sui.GetNetworkName(tt.network))
			require.Equal(t, tt.networkID, sui.GetNetworkID(tt.network))
			require.Equal(t, info.DefaultURLs, sui.GetDefaultURLs(tt.network))
		})
	}
}

func TestGetGaslessStablecoinType(t *testing.T) {
	coinType, ok := sui.GetGaslessStablecoinType("sui:mainnet", "USDC")
	require.True(t, ok)
	require.Equal(t, sui.USDCType, coinType)

	coinType, ok = sui.GetGaslessStablecoinType("sui:mainnet", sui.USDCType)
	require.True(t, ok)
	require.Equal(t, sui.USDCType, coinType)

	require.Len(t, sui.GetGaslessStablecoinTypes("sui:mainnet"), len(sui.DefaultGaslessStablecoinTypeList))
	_, ok = sui.GetGaslessStablecoinType("sui:mainnet", "NOT_A_TOKEN")
	require.False(t, ok)
	require.Nil(t, sui.GetNetworkInfo("sui:unknown"))
}
