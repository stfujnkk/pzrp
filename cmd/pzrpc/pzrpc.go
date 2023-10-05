package main

import (
	"context"
	"net"
	"pzrp/client"
	"pzrp/pkg/proto"
)

func main() {
	// ctx, cancel := context.WithCancel(context.Background())
	ctx := context.Background()
	con, err := net.Dial("tcp", "127.0.0.1:8848")
	if err != nil {
		panic(err)
	}
	tu := client.NewTunnelClientNode(con.(*net.TCPConn), ctx)
	tu.AddServer(proto.PROTO_TCP, 9200, 8100)
	tu.Run()
}
