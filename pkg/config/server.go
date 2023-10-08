package config

import (
	"encoding/json"
	"os"
)

type ServerConf struct {
	BindAddr string `json:"bind_addr"`
	BindPort uint8  `json:"bind_port"`
}

func LoadServerConfig() (*ServerConf, error) {
	var conf ServerConf
	jsonStr, err := os.ReadFile("s.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(jsonStr, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}
