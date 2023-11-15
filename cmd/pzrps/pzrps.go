package main

import (
	"context"
	"flag"
	"fmt"
	"pzrp/pkg/common"
	"pzrp/pkg/config"
	"pzrp/server"
)

func main() {
	configPath := flag.String("config", "pzrps.json", "configuration file path")
	showVersion := flag.Bool("version", false, "display version number")
	flag.Parse()
	if *showVersion {
		fmt.Println(common.VERSION)
		return
	}
	conf, err := config.LoadServerConfig(*configPath)
	if err != nil {
		panic(err)
	}
	server.Run(context.Background(), conf)
}
