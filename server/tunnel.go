package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"pzrp/pkg/config"
	"pzrp/pkg/proto"
	"pzrp/pkg/proto/tcp"
)

type TunnelNode struct {
	*tcp.TCPNode
	services map[uint8]map[uint16]proto.Node
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewTunnelNode(conn *net.TCPConn, ctx context.Context) *TunnelNode {
	_ctx, _cancel := context.WithCancel(ctx)
	node := &TunnelNode{
		TCPNode:  tcp.NewTCPNode(conn, _ctx, _ctx, false),
		services: map[uint8]map[uint16]proto.Node{},
		ctx:      _ctx,
		cancel:   _cancel,
	}
	node.Pack = node.overridePack
	node.UnPack = node.overrideUnPack
	return node
}

func (node *TunnelNode) AddServer(protocol uint8, port uint16) {
	services, ok := node.services[protocol]
	if !ok {
		services = make(map[uint16]proto.Node)
		node.services[protocol] = services
	}
	services[port] = nil
}

func (node *TunnelNode) initServer() {
	data := make([]byte, 0)
	for {
		msg, err := node.Read()
		if err != nil {
			panic(err)
		}
		data = append(data, msg.Data...)
		if len(msg.Data) < 0xffff {
			break
		}
	}
	conf := config.ClientConf{}
	json.Unmarshal(data, &conf)
	addServer := func(protocol uint8, port uint16, _ uint16) {
		node.AddServer(protocol, port)
	}
	config.RegisterService(&conf, addServer)
	for k, v := range node.services {
		switch k {
		case proto.PROTO_TCP:
			for port := range v {
				v[port] = node.startTCPServer(port)
				node.connectNode(v[port])
			}
		case proto.PROTO_UDP:
			panic(fmt.Errorf("unimplemented protocol:%v", k))
		default:
			panic(fmt.Errorf("unknown protocol:%v", k))
		}
	}
}

func (node *TunnelNode) Run() {
	defer func() {
		node.cancel()
	}()
	go node.initServer()
	node.TCPNode.Run()
}

func (node *TunnelNode) startTCPServer(port uint16) proto.Node {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		panic(err)
	}
	lis, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		panic(err)
	}
	srv := NewTCPServerNode(lis, node.ctx)
	go srv.Run() // TODO 运行失败后重启
	return srv
}

func (node *TunnelNode) connectNode(nextNode proto.Node) {
	go func() {
		for {
			msg, err := nextNode.Read()
			if err != nil {
				break
			}
			// node.inch <- msg
			err = node.Write(msg)
			if err != nil {
				break
			}
		}
	}()
	go func() {
		for {
			msg, err := node.Read()
			if err != nil {
				break
			}
			err = nextNode.Write(msg)
			if err != nil {
				break
			}
		}
	}()
}

func (node *TunnelNode) SetReadCtx(ctx context.Context) {
	node.setCtx(ctx)
}
func (node *TunnelNode) SetWriteCtx(ctx context.Context) {
	node.setCtx(ctx)
}

func (node *TunnelNode) setCtx(ctx context.Context) {
	_ctx, _cancel := context.WithCancel(ctx)
	node.ctx = _ctx
	node.cancel = _cancel
	node.TCPNode.SetReadCtx(node.ctx)
	node.TCPNode.SetWriteCtx(node.ctx)
}

func (node *TunnelNode) overridePack(msg *proto.Msg, data []byte) (int, error) {
	pkg, n, err := proto.NewPacket(data)
	if pkg != nil {
		*msg = *pkg.ToMsg()
	}
	return n, err
}

func (node *TunnelNode) overrideUnPack(msg proto.Msg) ([]byte, error) {
	data, err := msg.ToPacket().Encode()
	return data, err
}
