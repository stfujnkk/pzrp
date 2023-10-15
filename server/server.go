package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"pzrp/pkg/config"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"pzrp/pkg/proto/tcp"
	"pzrp/pkg/utils"
	"sync"
)

type TCPClientNode struct {
	*tcp.TCPNode
}

func NewTCPClientNode(conn tcp.DuplexConnection, readCtx, writeCtx context.Context) *TCPClientNode {
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

func newClientInfo(con tcp.DuplexConnection, ctx context.Context) *tcpClientInfo {
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
	logger := utils.GetLogger(node.ctx)
	defer func() {
		e := recover()
		if e != nil {
			logger.Error("abnormal service exit", "error", e)
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
	logger := utils.GetLogger(node.ctx)
	defer func() {
		e := recover()
		if e != nil {
			logger.Error("push message failed", "error", e)
		}
		node.cancel()
	}()
	for msg := range node.inch {
		node.dispatchMsg(msg)
	}
}

func (node *TCPServerNode) dispatchMsg(msg proto.Msg) {
	node.lock.Lock()
	defer func() {
		node.lock.Unlock()
	}()

	logger := utils.GetLoggerWithMsg(node.ctx, msg)
	raddr := fmt.Sprintf("%s:%d", msg.RemoteIP.String(), msg.RemotePort)
	info, ok := node.clientInfoMap[raddr]
	defer func() {
		if e := recover(); e != nil {
			logger.Warn("write error", "error", e)
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
			logger.Info("write off")
		}
		if (msg.Action & proto.ACTION_CLOSE_WRITE) != 0 {
			info.readCancel()
			logger.Info("read off")
		}
		return
	} else {
		if !ok {
			panic(fmt.Sprintf("address not found: %s", raddr))
		}
		err := info.node.Write(msg)
		if err != nil {
			panic(err)
		}
	}
}

func (node *TCPServerNode) notifyClose(ip string, port uint16, action uint8) {
	logger := utils.GetLogger(node.ctx)
	defer func() {
		if e := recover(); e != nil {
			logger.Error("notification close failed", "error", e)
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

func (node *TCPServerNode) collectFromClient(clientInfo *tcpClientInfo, logger *slog.Logger) {
	clientNode := clientInfo.node
	defer func() {
		e := recover()
		if e != nil {
			logger.Warn("failed to read data", "error", e)
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
	baseLogger := utils.GetLogger(node.ctx)
	for {
		con, err := node.listener.AcceptTCP()
		if err != nil {
			panic(err)
		}
		logger := baseLogger.With("remote_addr", con.RemoteAddr())
		logger.Info("accept new connections")
		info := node.registerConnection(con)
		go node.collectFromClient(info, logger)
	}
}

func (node *TCPServerNode) registerConnection(con tcp.DuplexConnection) *tcpClientInfo {
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

func getListener(conf *config.ServerConf) net.Listener {
	addr := fmt.Sprintf("%s:%d", conf.BindAddr, conf.BindPort)
	if conf.CertFile != "" && conf.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(conf.CertFile, conf.KeyFile)
		if err != nil {
			panic(err)
		}
		var certPool *x509.CertPool = nil
		if conf.CaCert != "" {
			certBytes, err := os.ReadFile(conf.CaCert)
			if err != nil {
				panic(err)
			}
			certPool = x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM(certBytes)
			if !ok {
				panic(certPool)
			}
		}
		tlsConf := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		}
		lis, err := tls.Listen("tcp", addr, tlsConf)
		if err != nil {
			panic(err)
		}
		return tcp.TlsListenerWrapper{Listener: lis}
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	return lis
}

func Run(ctx context.Context, conf *config.ServerConf) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
	}()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	baseCtx := utils.SetLogger(ctx, slog.Default())
	baseLogger := utils.GetLogger(baseCtx)
	lis := getListener(conf)
	go func() {
		<-ctx.Done()
		lis.Close()
	}()
	baseLogger.Info("successfully started service", "bind_addr", conf.BindAddr, "bind_port", conf.BindPort)
	defer func() {
		if e := recover(); e != nil {
			baseLogger.Error("service exit", "error", e)
		}
	}()
	for {
		con, err := lis.Accept()
		if err != nil {
			panic(err)
		}
		logger := baseLogger.With("client_addr", con.RemoteAddr())
		logger.Info("new client")
		subCtx := utils.SetLogger(ctx, logger)
		tun := NewTunnelNode(con.(tcp.DuplexConnection), subCtx)
		go tun.Run()
	}
}
