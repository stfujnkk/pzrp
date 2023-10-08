package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"pzrp/pkg/config"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"pzrp/pkg/proto/tcp"
	"sync"
)

type TCPClientNode struct {
	*tcp.TCPNode
}

func NewTCPClientNode(conn *net.TCPConn, readCtx, writeCtx context.Context) *TCPClientNode {
	node := &TCPClientNode{
		TCPNode: tcp.NewTCPNode(conn, readCtx, writeCtx, true),
	}
	node.Pack = node.overridePack
	node.UnPack = node.overrideUnPack
	return node
}

func (node *TCPClientNode) overridePack(msg *proto.Msg, data []byte) (int, error) {
	msg.ServerPort = node.ServerPort
	msg.RemoteIP = net.ParseIP(node.RemoteHost)
	msg.RemotePort = node.RemotePort
	msg.Protocol = proto.PROTO_TCP
	msg.Action = proto.ACTION_SEND_DATA
	msg.Data = data
	return len(data), nil
}

func (node *TCPClientNode) overrideUnPack(msg proto.Msg) ([]byte, error) {
	return msg.Data, nil
}

type tcpClientInfo struct {
	node        *TCPClientNode
	readCancel  context.CancelFunc
	writeCancel context.CancelFunc
	readCtx     context.Context
	writeCtx    context.Context
}

func newClientInfo(con *net.TCPConn, ctx context.Context) *tcpClientInfo {
	info := &tcpClientInfo{}
	info.readCtx, info.readCancel = context.WithCancel(ctx)
	info.writeCtx, info.writeCancel = context.WithCancel(ctx)
	info.node = NewTCPClientNode(con, info.readCtx, info.writeCtx)
	return info
}

type TCPServerNode struct {
	ctx           context.Context
	cancel        context.CancelFunc
	listener      *net.TCPListener
	lock          *sync.Mutex
	clientInfoMap map[string]*tcpClientInfo
	inch          chan proto.Msg
	ouch          chan proto.Msg
	closeOnce     *sync.Once
	ServerPort    uint16
}

func NewTCPServerNode(listener *net.TCPListener, ctx context.Context) *TCPServerNode {
	addr := listener.Addr().(*net.TCPAddr)
	node := &TCPServerNode{
		listener:      listener,
		lock:          new(sync.Mutex),
		clientInfoMap: map[string]*tcpClientInfo{},
		inch:          make(chan proto.Msg),
		ouch:          make(chan proto.Msg),
		closeOnce:     &sync.Once{},
		ServerPort:    uint16(addr.Port),
	}
	node.setCtx(ctx)
	return node
}

func (node *TCPServerNode) Run() {
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("tcp服务运行错误：%v\n", e)
		}
		node.cancel()
	}()
	go func() {
		<-node.ctx.Done()
		node.closeOnce.Do(node.closeServer)
	}()
	go node.startDispatch()
	node.startAccept()
}

func (node *TCPServerNode) startDispatch() {
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("推送消息错误：%v\n", e)
		}
		node.cancel()
	}()
	for msg := range node.inch {
		node.dispatchMsg(msg)
	}
}

func (node *TCPServerNode) dispatchMsg(msg proto.Msg) {
	raddr := fmt.Sprintf("%s:%d", msg.RemoteIP.String(), msg.RemotePort)
	info, ok := node.clientInfoMap[raddr]
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("写入%v错误：%v\n", msg, e)
			node.notifyClose(msg.RemoteIP.String(), msg.RemotePort, proto.ACTION_CLOSE_WRITE)
			if info != nil && info.readCancel != nil {
				info.writeCancel()
			}
		}
	}()
	if proto.IsCloseMsg(msg) {
		if !ok {
			return
		}
		if (msg.Action & proto.ACTION_CLOSE_READ) != 0 {
			info.writeCancel()
		}
		if (msg.Action & proto.ACTION_CLOSE_WRITE) != 0 {
			info.readCancel()
		}
		return
	} else {
		if !ok {
			panic(fmt.Sprintf("找不到 %s", raddr))
		}
		err := info.node.Write(msg)
		if err != nil {
			panic(err)
		}
	}
}

func (node *TCPServerNode) notifyClose(ip string, port uint16, action uint8) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("通知关闭时出错：%v\n", e)
			node.cancel()
		}
	}()

	node.ouch <- proto.Msg{
		RemoteIP:   net.ParseIP(ip),
		RemotePort: port,
		Action:     action,
		Protocol:   proto.PROTO_TCP,
		ServerPort: node.ServerPort,
		Data:       []byte{},
	}
}

func (node *TCPServerNode) collectFromClient(clientInfo *tcpClientInfo) {
	clientNode := clientInfo.node
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("读取%v错误：%v\n", clientNode, e)
		}
		node.notifyClose(clientNode.RemoteHost, clientNode.RemotePort, proto.ACTION_CLOSE_READ)
		if clientInfo != nil && clientInfo.readCancel != nil {
			clientInfo.readCancel()
		}
	}()
	for {
		msg, err := clientNode.Read()
		if err != nil {
			panic(err)
		}
		node.ouch <- msg
	}
}

func (node *TCPServerNode) startAccept() {
	for {
		con, err := node.listener.AcceptTCP()
		if err != nil {
			panic(err)
		}
		fmt.Printf("新连接%v\n", con)
		info := node.registerConnection(con)
		go node.collectFromClient(info)
	}
}

func (node *TCPServerNode) registerConnection(con *net.TCPConn) *tcpClientInfo {
	node.lock.Lock()
	defer func() {
		node.lock.Unlock()
	}()
	info := newClientInfo(con, node.ctx)
	clientNode := info.node
	raddr := fmt.Sprintf("%s:%d", clientNode.RemoteHost, clientNode.RemotePort)
	node.clientInfoMap[raddr] = info
	go func() {
		node.ouch <- proto.Msg{
			RemoteIP:   net.ParseIP(clientNode.RemoteHost),
			RemotePort: clientNode.RemotePort,
			Action:     proto.ACTION_SEND_DATA,
			Protocol:   proto.PROTO_TCP,
			ServerPort: node.ServerPort,
			Data:       []byte{},
		}
		clientNode.Run()
		node.lock.Lock()
		defer func() {
			node.lock.Unlock()
		}()
		delete(node.clientInfoMap, raddr)
	}()
	return info
}

func (node *TCPServerNode) Read() (proto.Msg, error) {
	msg, ok := <-node.ouch
	var err error = nil
	if !ok {
		err = pkgErr.ErrClosed
	}
	return msg, err
}
func (node *TCPServerNode) Write(msg proto.Msg) (err error) {
	defer (func() {
		if e := recover(); e != nil {
			err = pkgErr.ErrClosed
		}
	})()
	node.inch <- msg
	return nil
}

func (node *TCPServerNode) SetReadCtx(ctx context.Context) {
	node.setCtx(ctx)
}
func (node *TCPServerNode) SetWriteCtx(ctx context.Context) {
	node.setCtx(ctx)
}

func (node *TCPServerNode) setCtx(ctx context.Context) {
	_ctx, cancel := context.WithCancel(ctx)
	node.ctx = _ctx
	node.cancel = cancel
}

func (node *TCPServerNode) closeServer() {
	node.listener.Close()
	close(node.ouch)
	close(node.inch)
}

var _ proto.Node = &TCPServerNode{}

func Run() {
	conf, err := config.LoadServerConfig()
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)
	go func() {
		select {
		case sig := <-sigChan:
			fmt.Printf("用户强制退出：%v\n", sig)
			cancel()
		case <-ctx.Done():
		}
	}()
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", conf.BindAddr, conf.BindPort))
	if err != nil {
		panic(err)
	}
	for {
		con, err := lis.Accept()
		if err != nil {
			panic(err)
		}
		tun := NewTunnelNode(con.(*net.TCPConn), ctx)
		go tun.Run()
	}
}
