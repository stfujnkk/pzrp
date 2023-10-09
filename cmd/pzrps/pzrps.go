package main

import (
	"context"
	"flag"
	"pzrp/pkg/config"
	"pzrp/server"
)

func main() {
	configPath := flag.String("config", "pzrps.json", "configuration file path")
	flag.Parse()
	conf, err := config.LoadServerConfig(*configPath)
	if err != nil {
		panic(err)
	}
	server.Run(context.Background(), conf)
}
