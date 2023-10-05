package main

import (
	"context"
	"net"
	"pzrp/pkg/proto"
	"pzrp/server"
)

func main() {
	// ctx, cancel := context.WithCancel(context.Background())
	ctx := context.Background()
	lis, err := net.Listen("tcp", "0.0.0.0:8848")
	if err != nil {
		panic(err)
	}
	con, err := lis.Accept()
	if err != nil {
		panic(err)
	}
	tu := server.NewTunnelNode(con.(*net.TCPConn), ctx)
	tu.AddServer(proto.PROTO_TCP, 9200)
	tu.Run()
}
