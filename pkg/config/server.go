package config

import (
	"encoding/json"
	"os"
)

type ServerConf struct {
	BindAddr string `json:"bind_addr"`
	BindPort int    `json:"bind_port"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
	CaCert   string `json:"ca_cert"`
}

func LoadServerConfig(configPath string) (*ServerConf, error) {
	var conf ServerConf
	jsonStr, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(jsonStr, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}
