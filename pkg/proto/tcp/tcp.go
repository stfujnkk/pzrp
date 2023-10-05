package tcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"runtime"
	"sync"
	"time"
)

type TCPNode struct {
	*tcpNode
}

var _ proto.Node = &TCPNode{}

func NewTCPNode(conn *net.TCPConn, readCtx, writeCtx context.Context, isWait bool) *TCPNode {
	// conn.SetKeepAlive()
	// conn.SetKeepAlivePeriod()
	raddr := conn.RemoteAddr().(*net.TCPAddr)
	laddr := conn.LocalAddr().(*net.TCPAddr)
	node := &tcpNode{
		ServerPort: uint16(laddr.Port),
		RemoteHost: raddr.IP.String(),
		RemotePort: uint16(raddr.Port),
		inch:       make(chan proto.Msg),
		ouch:       make(chan proto.Msg),
		done:       make(chan struct{}),
		closed:     0,
		readError:  nil,
		writeError: nil,
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
		fmt.Println("gc")
		// 自动关闭资源
		n.closeRead(errors.New("gc"))
		n.closeWrite(errors.New("gc"))
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
	readError      any
	writeError     any
	lock           *sync.Mutex
	conn           *net.TCPConn
	Pack           func(msg *proto.Msg, data []byte) (int, error)
	UnPack         func(msg proto.Msg) ([]byte, error)
	readCtx        context.Context
	writeCtx       context.Context
	readCancel     context.CancelFunc
	writeCancel    context.CancelFunc
	isWait         bool
	closeWaitTimer *time.Timer
}

func (node *tcpNode) Run() {
	go func() {
		<-node.readCtx.Done()
		node.closeRead(pkgErr.ErrCanceled)
	}()
	go func() {
		<-node.writeCtx.Done()
		node.closeWrite(pkgErr.ErrCanceled)
	}()
	go node.startRead()
	go node.startWrite()
	<-node.Done()
	fmt.Printf("关闭节点%v\n", *node)
}

func (node *tcpNode) Write(msg proto.Msg) (err any) {
	defer (func() {
		if e := recover(); e != nil {
			err = pkgErr.ErrClosed
			if node.writeError != nil {
				err = node.writeError
			}
		}
	})()
	node.inch <- msg
	if node.isWait && (node.closed&proto.ACTION_CLOSE_READ) != 0 {
		node.resetCloseWaitTimer()
	}
	return nil
}

func (node *tcpNode) Read() (proto.Msg, any) {
	msg, ok := <-node.ouch
	var err any = nil
	if !ok {
		err = pkgErr.ErrClosed
		if node.readError != nil {
			err = node.readError
		}
	}
	return msg, err
}

func (node *tcpNode) SetReadCtx(ctx context.Context) {
	node.readCtx, node.readCancel = context.WithCancel(ctx)
	if !node.isWait {
		node.writeCtx, node.writeCancel = node.readCtx, node.readCancel
	}
}

func (node *tcpNode) SetWriteCtx(ctx context.Context) {
	// TODO use context.WithCancelCause()
	node.writeCtx, node.writeCancel = context.WithCancel(ctx)
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
		node.closeRead(e)
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
		node.closeWrite(e)
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
			node.closeWrite(errors.New("close_wait time out"))
		})
	}
}

func (node *tcpNode) closeRead(reason any) {
	node.lock.Lock()
	defer (func() {
		node.lock.Unlock()
	})()

	if (node.closed & proto.ACTION_CLOSE_READ) != 0 {
		return
	}
	node.closed |= proto.ACTION_CLOSE_READ
	if node.readError == nil && reason != nil {
		node.readError = reason
	}
	node.readCancel() // 释放goroutine
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
func (node *tcpNode) closeWrite(reason any) {
	node.lock.Lock()
	defer (func() {
		node.lock.Unlock()
	})()

	if (node.closed & proto.ACTION_CLOSE_WRITE) != 0 {
		return
	}
	node.closed |= proto.ACTION_CLOSE_WRITE
	if node.writeError == nil && reason != nil {
		node.writeError = reason
	}
	node.writeCancel() // 释放goroutine
	close(node.inch)
	if node.closed == proto.ACTION_CLOSE_ALL {
		close(node.done)
		node.conn.Close()
	} else {
		node.conn.CloseWrite()
	}
}
