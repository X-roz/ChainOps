package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ListenerConfig struct {
	Network string   `yaml:"network"`
	RPCURLs []string `yaml:"rpc-urls"`
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

	return &cfg, nil
}
