package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"pzrp/pkg/config"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"pzrp/pkg/proto/tcp"
	"pzrp/pkg/proto/udp"
	"pzrp/pkg/utils"
	"time"
)

type TunnelNode struct {
	*tcp.TCPNode
	services map[uint8]map[uint16]proto.Node
	ctx      context.Context
	cancel   context.CancelFunc
	key      []byte
}

func NewTunnelNode(conn tcp.DuplexConnection, ctx context.Context, key []byte) *TunnelNode {
	_ctx, _cancel := context.WithCancel(ctx)
	node := &TunnelNode{
		TCPNode:  tcp.NewTCPNode(conn, _ctx, _ctx, false),
		services: map[uint8]map[uint16]proto.Node{},
		ctx:      _ctx,
		cancel:   _cancel,
		key:      key,
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

func (node *TunnelNode) findServer(protocol uint8, serverPort uint16) proto.Node {
	s1, ok := node.services[protocol]
	if !ok {
		return nil
	}
	s2, ok := s1[serverPort]
	if !ok {
		return nil
	}
	return s2
}

func (node *TunnelNode) dispatchMsg() {
	logger := utils.GetLogger(node.ctx)
	defer func() {
		e := recover()
		if e != nil {
			logger.Error("dispatch message failed", "error", e)
		}
		node.cancel()
	}()
	for {
		msg, err := node.Read()
		if err != nil {
			panic(err)
		}
		nextNode := node.findServer(msg.Protocol, msg.ServerPort)
		err = nextNode.Write(msg)
		if err != nil {
			logger.Error("dispatch message failed", "error", err, "node", nextNode)
		}
	}
}

func (node *TunnelNode) collectMsg(server proto.Node) {
	logger := utils.GetLogger(node.ctx)
	defer func() {
		e := recover()
		if e != nil {
			logger.Error("collect message failed", "error", e)
		}
		node.cancel()
	}()
	for {
		msg, err := server.Read()
		if err != nil {
			panic(err)
		}
		err = node.Write(msg)
		if err != nil {
			panic(err)
		}
	}
}

func (node *TunnelNode) sendAuthRequest() {
	if node.key == nil || len(node.key) == 0 {
		return
	}
	authMsg := proto.Msg{
		RemoteIP:   net.ParseIP("0.0.0.0"),
		RemotePort: 0,
		Protocol:   proto.PROTO_NUL,
		ServerPort: 0,
		Action:     proto.ACTION_AUTH,
		Data:       make([]byte, 64),
	}
	_, err := rand.Read(authMsg.Data)
	if err != nil {
		panic(err)
	}
	msg := make([]byte, len(authMsg.Data))
	copy(msg, authMsg.Data)

	err = node.Write(authMsg)
	if err != nil {
		panic(err)
	}
	authMsg, err = node.Read()
	if err != nil {
		panic(err)
	}
	if authMsg.Action != proto.ACTION_AUTH {
		panic(pkgErr.ErrAuth)
	}
	if !proto.Auth(node.key, msg, authMsg.Data) {
		panic(pkgErr.ErrAuth)
	}
}

func (node *TunnelNode) initServer() {
	logger := utils.GetLogger(node.ctx)
	timeOutTimer := time.NewTimer(3 * time.Second)
	isOk := make(chan struct{})
	defer func() {
		timeOutTimer.Stop()
		if e := recover(); e != nil {
			logger.Error("failed to initialize service", "error", e)
			node.cancel()
		}
	}()
	go func() {
		select {
		case <-timeOutTimer.C:
			node.cancel()
		case <-isOk:
		}
	}()
	node.sendAuthRequest()
	data := make([]byte, 0)
	for {
		msg, err := node.Read()
		if err != nil {
			panic(err)
		}
		if msg.Action != proto.ACTION_SET_CONFIG {
			panic(pkgErr.ErrAbnormalPacket)
		}
		data = append(data, msg.Data...)
		if len(msg.Data) < 0xffff {
			break
		}
	}
	conf := config.ClientConf{}
	json.Unmarshal(data, &conf)
	config.RegisterService(&conf,
		func(protocol uint8, port uint16, _ string, _ uint16) {
			node.AddServer(protocol, port)
		},
	)
	for k, v := range node.services {
		switch k {
		case proto.PROTO_TCP:
			for port := range v {
				v[port] = node.startTCPServer(port)
				go node.collectMsg(v[port])
			}
		case proto.PROTO_UDP:
			for port := range v {
				v[port] = node.startUDPServer(port)
				go node.collectMsg(v[port])
			}
		default:
			panic(fmt.Errorf("unknown protocol:%v", k))
		}
	}
	close(isOk)
	logger.Info("service startup completed")
	node.dispatchMsg()
}

func (node *TunnelNode) Run() {
	logger := utils.GetLogger(node.ctx)
	defer func() {
		e := recover()
		if e != nil {
			logger.Error("tunnel abnormal exit", "error", e)
		}
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
	baseLogger := utils.GetLogger(node.ctx)
	logger := baseLogger.With("protocol", "tcp", "server_port", port)
	ctx := utils.SetLogger(node.ctx, logger)
	srv := NewTCPServerNode(lis, ctx)
	go srv.Run() // TODO 运行失败后重启
	return srv
}

func (node *TunnelNode) startUDPServer(port uint16) proto.Node {
	server, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: int(port),
	})
	if err != nil {
		panic(err)
	}
	srv := udp.NewUdpServerNode(server, port, node.ctx)
	go srv.Run()
	return srv
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
