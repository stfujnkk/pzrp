package server

import (
	"context"
	"fmt"
	"net"
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

type clientInfo struct {
	readCancel  context.CancelFunc
	writeCancel context.CancelFunc
	readCtx     context.Context
	writeCtx    context.Context
}

func newClientInfo(ctx context.Context) *clientInfo {
	info := &clientInfo{}
	info.readCtx, info.readCancel = context.WithCancel(ctx)
	info.writeCtx, info.writeCancel = context.WithCancel(ctx)
	return info
}

type TCPServerNode struct {
	ctx           context.Context
	cancel        context.CancelFunc
	listener      *net.TCPListener
	lock          *sync.Mutex
	clients       map[string]*TCPClientNode
	clientInfoMap map[string]*clientInfo
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
		clients:       map[string]*TCPClientNode{},
		clientInfoMap: map[string]*clientInfo{},
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
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("写入%v错误：%v\n", msg, e)
			node.notifyClose(msg.RemoteIP.String(), msg.RemotePort, proto.ACTION_CLOSE_WRITE)
		}
	}()
	raddr := fmt.Sprintf("%s:%d", msg.RemoteIP.String(), msg.RemotePort)
	if proto.IsCloseMsg(msg) {
		info, ok := node.clientInfoMap[raddr]
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
		client, ok := node.clients[raddr]
		if !ok {
			panic(fmt.Sprintf("找不到 %s", raddr))
		}
		err := client.Write(msg)
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
	// raddr := fmt.Sprintf("%s:%d", ip, port)
	// if (action & proto.ACTION_CLOSE_WRITE) != 0 {
	// 	delete(node.clients, raddr)
	// }
	node.ouch <- proto.Msg{
		RemoteIP:   net.ParseIP(ip),
		RemotePort: port,
		Action:     action,
		Protocol:   proto.PROTO_TCP,
		ServerPort: node.ServerPort,
		Data:       []byte{},
	}
}

func (node *TCPServerNode) collectFromClient(clientNode *TCPClientNode) {
	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("读取%v错误：%v\n", clientNode, e)
		}
		node.notifyClose(clientNode.RemoteHost, clientNode.RemotePort, proto.ACTION_CLOSE_READ)
		// TODO 关闭clientNode的read
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
		clientNode := node.registerConnection(con)
		go node.collectFromClient(clientNode)
	}
}

func (node *TCPServerNode) registerConnection(con *net.TCPConn) *TCPClientNode {
	node.lock.Lock()
	defer func() {
		node.lock.Unlock()
	}()
	info := newClientInfo(node.ctx)
	clientNode := NewTCPClientNode(con, info.readCtx, info.writeCtx)
	raddr := fmt.Sprintf("%s:%d", clientNode.RemoteHost, clientNode.RemotePort)
	node.clients[raddr] = clientNode
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
		delete(node.clients, raddr)
		delete(node.clientInfoMap, raddr)
	}()
	return clientNode
}

func (node *TCPServerNode) Read() (proto.Msg, any) {
	msg, ok := <-node.ouch
	var err any = nil
	if !ok {
		err = pkgErr.ErrClosed
	}
	return msg, err
}
func (node *TCPServerNode) Write(msg proto.Msg) (err any) {
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
