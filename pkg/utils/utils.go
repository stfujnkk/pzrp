package utils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"pzrp/pkg/proto"
	"time"
)

func TCPipe(port1 int, port2 int, delay time.Duration) (*net.TCPConn, *net.TCPConn) {
	addr1 := fmt.Sprintf("127.0.0.1:%d", port1)
	addr2 := fmt.Sprintf("127.0.0.1:%d", port2)
	listener, err := net.Listen("tcp", addr1)
	if err != nil {
		fmt.Printf("listen fail, err: %v\n", err)
		return nil, nil
	}
	ch := make(chan net.Conn)
	go (func(ch chan<- net.Conn) {
		time.Sleep(delay)
		conn2, err := net.Dial("tcp", addr2)
		if err != nil {
			fmt.Printf("dial fail, err: %v\n", err)
			ch <- nil
			return
		}
		ch <- conn2
	})(ch)
	conn1, err := listener.Accept()
	if err != nil {
		fmt.Printf("accept fail, err: %v\n", err)
		return nil, nil
	}
	conn2 := <-ch
	return conn1.(*net.TCPConn), conn2.(*net.TCPConn)
}

func UDPipe(port1 int, port2 int, delay time.Duration) (*net.UDPConn, *net.UDPConn) {
	conn1, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: port1,
	})
	if err != nil {
		fmt.Printf("read from connect failed, err:%v\n", err)
		return nil, nil
	}
	ch := make(chan *net.UDPConn)
	go (func(ch chan<- *net.UDPConn) {
		time.Sleep(delay)
		conn2, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: port2,
		})
		if err != nil {
			fmt.Printf("DialUDP failed, err:%v\n", err)
			ch <- nil
			return
		}
		ch <- conn2
	})(ch)
	conn2 := <-ch
	return conn1, conn2
}

func NewErr(e any) error {
	err, ok := e.(error)
	if ok {
		return err
	}
	errMsg, ok := e.(string)
	if ok {
		return errors.New(errMsg)
	}
	return fmt.Errorf("%T : %+v", e, e)
}

// log

type key string

const (
	logKey key = "logger"
)

func GetLogger(ctx context.Context) *slog.Logger {
	return ctx.Value(logKey).(*slog.Logger)
}

func SetLogger(ctx context.Context, log *slog.Logger) context.Context {
	if log == nil {
		return ctx
	}
	return context.WithValue(ctx, logKey, log)
}

func GetLoggerWithMsg(ctx context.Context, msg proto.Msg) *slog.Logger {
	logger := GetLogger(ctx)
	return logger.With("remote_ip", msg.RemoteIP, "remote_port", msg.RemotePort, "protocol", proto.ProtoToStr[msg.Protocol])
}
