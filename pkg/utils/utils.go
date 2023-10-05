package utils

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
)

func TCPipe() (*net.TCPConn, *net.TCPConn) {
	port := rand.Intn(30000) + 10000
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("listen fail, err: %v\n", err)
		return nil, nil
	}
	ch := make(chan net.Conn)
	go (func(ch chan<- net.Conn) {
		conn2, err := net.Dial("tcp", addr)
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

func UDPipe() (*net.UDPConn, *net.UDPConn) {
	port := rand.Intn(30000) + 10000
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		fmt.Printf("can't resolve address, err:%v\n", err)
		return nil, nil
	}
	conn1, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Printf("read from connect failed, err:%v\n", err)
		return nil, nil
	}
	ch := make(chan *net.UDPConn)
	go (func(ch chan<- *net.UDPConn) {
		conn2, err := net.DialUDP("udp", nil, udpAddr)
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
