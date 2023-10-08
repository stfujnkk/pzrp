package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"pzrp/pkg/config"
	"pzrp/pkg/proto"
	"pzrp/pkg/proto/tcp"
	"sync"
)

type clientNodeInfo struct {
	node        proto.Node
	readCancel  context.CancelFunc
	writeCancel context.CancelFunc
	readCtx     context.Context
	writeCtx    context.Context
}

type TunnelClientNode struct {
	*tcp.TCPNode
	serviceMapping map[uint8]map[uint16]uint16 // 远程端口-->本地端口
	clients        map[uint8]map[uint16]map[string]*clientNodeInfo
	ctx            context.Context
	cancel         context.CancelFunc
	lock           *sync.Mutex
	conf           *config.ClientConf
}

func (node *TunnelClientNode) overridePack(msg *proto.Msg, data []byte) (int, error) {
	pkg, n, err := proto.NewPacket(data)
	if pkg != nil {
		*msg = *pkg.ToMsg()
	}
	return n, err
}

func (node *TunnelClientNode) overrideUnPack(msg proto.Msg) ([]byte, error) {
	data, err := msg.ToPacket().Encode()
	return data, err
}

func NewTunnelClientNode(conn *net.TCPConn, ctx context.Context, conf *config.ClientConf) *TunnelClientNode {
	_ctx, _cancel := context.WithCancel(ctx)
	node := &TunnelClientNode{
		TCPNode:        tcp.NewTCPNode(conn, _ctx, _ctx, false),
		cancel:         _cancel,
		ctx:            _ctx,
		serviceMapping: map[uint8]map[uint16]uint16{},
		clients:        map[uint8]map[uint16]map[string]*clientNodeInfo{},
		lock:           new(sync.Mutex),
		conf:           conf,
	}
	node.Pack = node.overridePack
	node.UnPack = node.overrideUnPack
	return node
}

func (node *TunnelClientNode) AddServer(protocol uint8, rPort uint16, lPort uint16) {
	services, ok := node.serviceMapping[protocol]
	if !ok {
		services = map[uint16]uint16{}
		node.serviceMapping[protocol] = services
	}
	_, ok = services[rPort]
	if ok {
		panic(fmt.Sprintf("重复分配端口:%d", rPort))
	}
	services[rPort] = lPort
}

func (node *TunnelClientNode) Run() {
	defer func() {
		node.cancel()
	}()
	go node.startDispatch()
	node.TCPNode.Run()
}

func (node *TunnelClientNode) pushConfig() {
	config.RegisterService(node.conf, node.AddServer)
	data, err := json.Marshal(node.conf)
	if err != nil {
		panic(err)
	}
	size := len(data)
	for i := 0; ; i += 0xffff {
		j := min(size, i+0xffff)
		msg := proto.Msg{
			RemoteIP: net.ParseIP("0.0.0.0"),
			Action:   proto.ACTION_SET_CONFIG,
			Data:     data[i:j],
		}
		node.TCPNode.Write(msg)
		if j == size {
			if len(msg.Data) == 0xffff {
				msg.Data = []byte{}
				node.TCPNode.Write(msg)
			}
			break
		}
	}
}

func (node *TunnelClientNode) startDispatch() {
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("read tunnel error: %v\n", e)
		}
		node.cancel()
	}()
	node.pushConfig()
	for {
		msg, err := node.Read()
		if err != nil {
			panic(err)
		}
		node.dispatchMsg(msg)
	}
}

func (node *TunnelClientNode) dispatchMsg(msg proto.Msg) {
	info, err := node.findClientNode(msg)
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("推送消息错误：%v\n", e)
			msg.Data = []byte{}
			msg.Action = proto.ACTION_CLOSE_WRITE
			node.Write(msg)
			if info != nil && info.writeCancel != nil {
				info.writeCancel()
			}
		}
	}()
	if proto.IsCloseMsg(msg) {
		if err != nil {
			fmt.Println(err)
		} else {
			if (msg.Action & proto.ACTION_CLOSE_READ) != 0 {
				info.writeCancel()
			}
			if (msg.Action & proto.ACTION_CLOSE_WRITE) != 0 {
				info.readCancel()
			}
		}
	} else {
		if err != nil {
			node.startConnect(msg)
		} else {
			err := info.node.Write(msg)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (node *TunnelClientNode) findClientNode(msg proto.Msg) (*clientNodeInfo, error) {
	_, ok := node.serviceMapping[msg.Protocol][msg.ServerPort]
	if !ok {
		return nil, fmt.Errorf("不存在对应的本地服务")
	}
	c1, ok := node.clients[msg.Protocol]
	if !ok {
		return nil, errors.New("找不到对应客户端")
	}
	c2, ok := c1[msg.ServerPort]
	if !ok {
		return nil, errors.New("找不到对应客户端")
	}
	addr := fmt.Sprintf("%s:%d", msg.RemoteIP.String(), msg.RemotePort)
	info, ok := c2[addr]
	if !ok {
		return nil, errors.New("找不到对应客户端")
	}
	return info, nil
}

func (node *TunnelClientNode) startConnect(msg proto.Msg) {
	node.lock.Lock()
	defer func() {
		node.lock.Unlock()
	}()
	info, err := node.findClientNode(msg)
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("连接本地服务失败：%v\n", e)
			msg.Data = []byte{}
			msg.Action = proto.ACTION_CLOSE_ALL
			node.Write(msg)
			if info != nil {
				if info.writeCancel != nil {
					info.writeCancel()
				}
				if info.readCancel != nil {
					info.readCancel()
				}
			}
		}
	}()
	if err != nil {
		// 找不到时创建一个新的
		switch msg.Protocol {
		case proto.PROTO_TCP:
			info = node.connectTCP(msg)
		case proto.PROTO_UDP:
			panic(fmt.Errorf("unimplemented protocol:%v", msg.Protocol))
		default:
			panic(fmt.Errorf("unknown protocol:%v", msg.Protocol))
		}
	}
	raddr := fmt.Sprintf("%s:%d", msg.RemoteIP.String(), msg.RemotePort)
	var (
		// 防止msg不gc
		serverPort = msg.ServerPort
		protocol   = msg.Protocol
	)
	node.registerConnection(protocol, serverPort, raddr, info)
	go func() {
		info.node.Run()
		node.removeConnection(protocol, serverPort, raddr)
	}()
	go node.collectMsgFromNode(info, protocol, serverPort, msg.RemoteIP, msg.RemotePort)
	e := info.node.Write(msg)
	if e != nil {
		panic(e)
	}
}

func (node *TunnelClientNode) collectMsgFromNode(info *clientNodeInfo, protocol uint8, sPort uint16, rIP net.IP, rPort uint16) {
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("读取%v失败：%v\n", info, e)
		}
		// 防止未关闭
		info.readCancel()
		// 通知远端关闭
		node.Write(proto.Msg{
			RemoteIP:   rIP,
			RemotePort: rPort,
			Action:     proto.ACTION_CLOSE_READ,
			Protocol:   protocol,
			ServerPort: sPort,
			Data:       []byte{},
		})
	}()
	for {
		msg, err := info.node.Read()
		if err != nil {
			panic(err)
		}
		err = node.Write(msg)
		if err != nil {
			// node.cancel()
			panic(err)
		}
	}
}

func (node *TunnelClientNode) registerConnection(protocol uint8, serverPort uint16, remoteAddr string, info *clientNodeInfo) {
	c1, ok := node.clients[protocol]
	if !ok {
		c1 = map[uint16]map[string]*clientNodeInfo{}
		node.clients[protocol] = c1
	}
	c2, ok := c1[serverPort]
	if !ok {
		c2 = map[string]*clientNodeInfo{}
		c1[serverPort] = c2
	}
	c2[remoteAddr] = info
}

func (node *TunnelClientNode) removeConnection(protocol uint8, serverPort uint16, remoteAddr string) {
	node.lock.Lock()
	defer func() {
		node.lock.Unlock()
	}()
	c1, ok := node.clients[protocol]
	if !ok {
		return
	}
	c2, ok := c1[serverPort]
	if !ok {
		return
	}
	delete(c2, remoteAddr)
}

func (node *TunnelClientNode) connectTCP(msg proto.Msg) *clientNodeInfo {
	localPort, ok := node.serviceMapping[msg.Protocol][msg.ServerPort]
	if !ok {
		panic(fmt.Errorf("不存在对应的本地服务"))
	}
	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		panic(err)
	}
	con, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		panic(err)
	}
	info := &clientNodeInfo{}
	info.readCtx, info.readCancel = context.WithCancel(node.ctx)
	info.writeCtx, info.writeCancel = context.WithCancel(node.ctx)
	info.node = NewProxyTCPNode(
		con, info.readCtx, info.writeCtx,
		msg.ServerPort, msg.RemoteIP, msg.RemotePort,
	)
	return info
}

type ProxyTCPNode struct {
	*tcp.TCPNode
	rIP   net.IP
	rPort uint16
	sProt uint16
}

func NewProxyTCPNode(
	conn *net.TCPConn,
	readCtx, writeCtx context.Context,
	ServerPort uint16,
	RemoteIP net.IP,
	RemotePort uint16,
) *ProxyTCPNode {
	node := &ProxyTCPNode{
		TCPNode: tcp.NewTCPNode(conn, readCtx, writeCtx, true),
		rIP:     RemoteIP,
		rPort:   RemotePort,
		sProt:   ServerPort,
	}
	node.Pack = node.overridePack
	node.UnPack = node.overrideUnPack
	return node
}

func (node *ProxyTCPNode) overridePack(msg *proto.Msg, data []byte) (int, error) {
	msg.ServerPort = node.sProt
	msg.RemoteIP = node.rIP
	msg.RemotePort = node.rPort
	msg.Protocol = proto.PROTO_TCP
	msg.Action = proto.ACTION_SEND_DATA
	msg.Data = data
	return len(data), nil
}

func (node *ProxyTCPNode) overrideUnPack(msg proto.Msg) ([]byte, error) {
	return msg.Data, nil
}

func Run() {
	conf, err := config.LoadClientConfig()
	if err != nil {
		panic(err)
	}
	con, err := net.Dial("tcp", fmt.Sprintf("%s:%d", conf.ServerAddr, conf.ServerPort))
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	tun := NewTunnelClientNode(con.(*net.TCPConn), ctx, conf)
	tun.Run()
}
