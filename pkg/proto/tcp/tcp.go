package tcp

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"pzrp/pkg/utils"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DuplexConnection interface {
	net.Conn
	CloseRead() error
	CloseWrite() error
}

type TCPNode struct {
	*tcpNode
}

var _ proto.Node = &TCPNode{}

func NewTCPNode(conn DuplexConnection, readCtx, writeCtx context.Context, isWait bool) *TCPNode {
	raddr := strings.Split(conn.RemoteAddr().String(), ":")
	rport, err := strconv.Atoi(raddr[1])
	if err != nil {
		panic(err)
	}
	laddr := strings.Split(conn.LocalAddr().String(), ":")
	lport, err := strconv.Atoi(laddr[1])
	if err != nil {
		panic(err)
	}
	node := &tcpNode{
		ServerPort: uint16(lport),
		RemoteHost: raddr[0],
		RemotePort: uint16(rport),
		inch:       make(chan proto.Msg),
		ouch:       make(chan proto.Msg),
		done:       make(chan struct{}),
		closed:     0,
		lock:       new(sync.Mutex),
		conn:       conn,
		Pack:       nil,
		UnPack:     nil,
		isWait:     isWait,
	}
	node.SetReadCtx(readCtx)
	node.SetWriteCtx(writeCtx)
	result := &TCPNode{node}
	runtime.SetFinalizer(result, func(n *TCPNode) {
		n.tcpNode.readCancel(pkgErr.ErrFreeByGC)
		n.tcpNode.writeCancel(pkgErr.ErrFreeByGC)
	})
	return result
}

type tcpNode struct {
	ServerPort     uint16
	RemoteHost     string
	RemotePort     uint16
	inch           chan proto.Msg
	ouch           chan proto.Msg
	done           chan struct{}
	closed         uint8
	lock           *sync.Mutex
	conn           DuplexConnection
	Pack           func(msg *proto.Msg, data []byte) (int, error)
	UnPack         func(msg proto.Msg) ([]byte, error)
	readCtx        context.Context
	writeCtx       context.Context
	readCancel     context.CancelCauseFunc
	writeCancel    context.CancelCauseFunc
	isWait         bool
	closeWaitTimer *time.Timer
}

func (node *tcpNode) Run() {
	go func() {
		<-node.readCtx.Done()
		node.closeRead()
	}()
	go func() {
		<-node.writeCtx.Done()
		node.closeWrite()
	}()
	go node.startRead()
	go node.startWrite()
	<-node.Done()
}

func (node *tcpNode) Write(msg proto.Msg) (err error) {
	defer (func() {
		if e := recover(); e != nil {
			err = context.Cause(node.writeCtx)
			if err == nil {
				err = utils.NewErr(e)
			}
		}
	})()
	node.inch <- msg
	if node.isWait && (node.closed&proto.ACTION_CLOSE_READ) != 0 {
		node.resetCloseWaitTimer()
	}
	return nil
}

func (node *tcpNode) Read() (proto.Msg, error) {
	msg, ok := <-node.ouch
	var err error = nil
	if !ok {
		err = context.Cause(node.readCtx)
		if err == nil {
			err = pkgErr.ErrClosed
		}
	}
	return msg, err
}

func (node *tcpNode) SetReadCtx(ctx context.Context) {
	node.readCtx, node.readCancel = context.WithCancelCause(ctx)
	if !node.isWait {
		node.writeCtx, node.writeCancel = node.readCtx, node.readCancel
	}
}

func (node *tcpNode) SetWriteCtx(ctx context.Context) {
	node.writeCtx, node.writeCancel = context.WithCancelCause(ctx)
	if !node.isWait {
		node.readCtx, node.readCancel = node.writeCtx, node.writeCancel
	}
}

func (node *tcpNode) Done() <-chan struct{} {
	return node.done
}

// 内部方法
func (node *tcpNode) startRead() {
	defer (func() {
		e := recover()
		if e != nil {
			node.readCancel(utils.NewErr(e))
		} else {
			node.readCancel(pkgErr.ErrClosed)
		}
	})()
	buf := make([]byte, 0)
	msg := proto.Msg{}
	data := make([]byte, 0xfff)
	for {
		n, err := node.conn.Read(data)
		if err != nil { // EOF
			panic(err)
		}
		buf = append(buf, data[:n]...)
		for {
			n, err = node.Pack(&msg, buf)
			if err != nil {
				panic(err)
			}
			if n == 0 {
				break
			}
			node.ouch <- msg
			buf = buf[n:]
			msg = proto.Msg{}
		}
	}
}

func (node *tcpNode) startWrite() {
	defer (func() {
		e := recover()
		if e != nil {
			node.writeCancel(utils.NewErr(e))
		} else {
			node.writeCancel(pkgErr.ErrClosed)
		}
	})()
	for msg := range node.inch {
		data, err := node.UnPack(msg)
		if err != nil {
			panic(err)
		}

		for len(data) > 0 {
			n, err := node.conn.Write(data)
			if err != nil {
				panic(err)
			}
			data = data[n:]
		}
	}
}

func (node *tcpNode) resetCloseWaitTimer() {
	isClosing := false
	if node.closeWaitTimer != nil {
		isClosing = !node.closeWaitTimer.Stop()
	}
	if !isClosing {
		node.closeWaitTimer = time.AfterFunc(60*time.Second, func() {
			node.writeCancel(pkgErr.ErrCloseWaitTimeOut)
		})
	}
}

func (node *tcpNode) closeRead() {
	node.lock.Lock()
	defer (func() {
		node.lock.Unlock()
	})()

	if (node.closed & proto.ACTION_CLOSE_READ) != 0 {
		return
	}
	node.closed |= proto.ACTION_CLOSE_READ
	close(node.ouch)
	if node.isWait {
		node.resetCloseWaitTimer()
	}
	if node.closed == proto.ACTION_CLOSE_ALL {
		close(node.done)
		node.conn.Close()
	} else {
		node.conn.CloseRead()
	}
}

func (node *tcpNode) closeWrite() {
	node.lock.Lock()
	defer (func() {
		node.lock.Unlock()
	})()

	if (node.closed & proto.ACTION_CLOSE_WRITE) != 0 {
		return
	}
	node.closed |= proto.ACTION_CLOSE_WRITE
	close(node.inch)
	if node.closed == proto.ACTION_CLOSE_ALL {
		close(node.done)
		node.conn.Close()
	} else {
		node.conn.CloseWrite()
	}
}

type TlsConWrapper struct {
	*tls.Conn
	readClosed bool
}

func (con *TlsConWrapper) Read(b []byte) (int, error) {
	if con.readClosed {
		return 0, io.EOF
	}
	return con.Conn.Read(b)
}

func (con *TlsConWrapper) CloseRead() error {
	con.readClosed = true
	return nil
}

type TlsListenerWrapper struct {
	net.Listener
}

func (lis TlsListenerWrapper) Accept() (net.Conn, error) {
	con, err := lis.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &TlsConWrapper{Conn: con.(*tls.Conn)}, nil
}
