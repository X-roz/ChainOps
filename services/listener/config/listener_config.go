package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
	Timezone string `yaml:"timezone"`
}

type ListenerConfig struct {
	Network             string         `yaml:"network"`
	RPCURLs             []string       `yaml:"rpc-urls"`
	NativeAsset         string         `yaml:"native-asset"`
	SafeBlockBuffer     int64          `yaml:"safe-block-buffer"`
	MaxBlocksPerTick    int64          `yaml:"max-blocks-per-tick"`
	EvmBlockListen      bool           `yaml:"evm-block-listen"`
	UsdcListen          bool           `yaml:"usdc-listen"`
	KnownTokenContracts []string       `yaml:"known-token-contracts"`
	Database            DatabaseConfig `yaml:"database"`
}

func Load(path string) (*ListenerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ListenerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Always load network in lowercase
	cfg.Network = strings.ToLower(cfg.Network)

	return &cfg, nil
}
