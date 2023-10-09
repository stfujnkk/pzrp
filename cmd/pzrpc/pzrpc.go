package main

import (
	"context"
	"flag"
	"pzrp/client"
	"pzrp/pkg/config"
)

func main() {
	configPath := flag.String("config", "pzrpc.json", "configuration file path")
	flag.Parse()
	conf, err := config.LoadClientConfig(*configPath)
	if err != nil {
		panic(err)
	}
	client.Run(context.Background(), conf)
}
