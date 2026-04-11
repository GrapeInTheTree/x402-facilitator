package evm

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGetDomainConfig(t *testing.T) {
	const (
		baseSepolia     = "eip155:84532"
		usdcBaseSepolia = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
	)

	t.Run("resolves by token symbol", func(t *testing.T) {
		cfg := GetDomainConfig(baseSepolia, "USDC")
		require.NotNil(t, cfg, "symbol lookup should succeed for known token")
		require.Equal(t, common.HexToAddress(usdcBaseSepolia), cfg.VerifyingContract)
		require.Equal(t, "USDC", cfg.Name)
	})

	t.Run("resolves by contract address", func(t *testing.T) {
		cfg := GetDomainConfig(baseSepolia, usdcBaseSepolia)
		require.NotNil(t, cfg, "address lookup should succeed for known token")
		require.Equal(t, common.HexToAddress(usdcBaseSepolia), cfg.VerifyingContract)
		require.Equal(t, "USDC", cfg.Name)
	})

	t.Run("address lookup is case-insensitive", func(t *testing.T) {
		cfg := GetDomainConfig(baseSepolia, strings.ToLower(usdcBaseSepolia))
		require.NotNil(t, cfg, "lowercase address should still resolve")
		require.Equal(t, common.HexToAddress(usdcBaseSepolia), cfg.VerifyingContract)
	})

	t.Run("symbol lookup and address lookup return the same config", func(t *testing.T) {
		bySymbol := GetDomainConfig(baseSepolia, "USDC")
		byAddress := GetDomainConfig(baseSepolia, usdcBaseSepolia)
		require.NotNil(t, bySymbol)
		require.NotNil(t, byAddress)
		require.Equal(t, bySymbol.VerifyingContract, byAddress.VerifyingContract)
		require.Equal(t, bySymbol.Name, byAddress.Name)
		require.Equal(t, bySymbol.Version, byAddress.Version)
		require.Equal(t, bySymbol.ChainID.String(), byAddress.ChainID.String())
	})

	t.Run("returns nil for unknown chain", func(t *testing.T) {
		require.Nil(t, GetDomainConfig("eip155:999999", "USDC"))
		require.Nil(t, GetDomainConfig("eip155:999999", usdcBaseSepolia))
	})

	t.Run("returns nil for unknown symbol on known chain", func(t *testing.T) {
		require.Nil(t, GetDomainConfig(baseSepolia, "WETH"))
	})

	t.Run("returns nil for unknown contract address on known chain", func(t *testing.T) {
		require.Nil(t, GetDomainConfig(baseSepolia, "0x0000000000000000000000000000000000000000"))
	})
}
