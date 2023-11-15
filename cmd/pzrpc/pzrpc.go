package main

import (
	"context"
	"flag"
	"fmt"
	"pzrp/client"
	"pzrp/pkg/common"
	"pzrp/pkg/config"
)

func main() {
	configPath := flag.String("config", "pzrpc.json", "configuration file path")
	showVersion := flag.Bool("version", false, "display version number")
	flag.Parse()
	if *showVersion {
		fmt.Println(common.VERSION)
		return
	}
	conf, err := config.LoadClientConfig(*configPath)
	if err != nil {
		panic(err)
	}
	client.Run(context.Background(), conf)
}
