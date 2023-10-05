package proto

import (
	"context"
	"net"
)

// 协议
const (
	PROTO_TCP = uint8(iota)
	PROTO_UDP = uint8(iota)
)

// 指令
const (
	ACTION_SEND_DATA   = uint8(iota)
	ACTION_CLOSE_READ  = uint8(iota)
	ACTION_CLOSE_WRITE = uint8(iota)
	ACTION_CLOSE_ALL   = ACTION_CLOSE_READ | ACTION_CLOSE_WRITE
)

type Node interface {
	Read() (Msg, any)
	Write(Msg) any
	SetReadCtx(context.Context)  // 通知节点关闭Read
	SetWriteCtx(context.Context) // 通知节点关闭Write
	Run()
}

type Msg struct {
	RemoteIP   net.IP
	RemotePort uint16
	Action     uint8
	Protocol   uint8
	ServerPort uint16
	Data       []byte
}

type Packet struct {
	Length uint16
	PacketHead
	Data   []byte
}

type PacketHead struct {
	RemoteIP   [4]byte
	RemotePort uint16
	Action     uint8
	Protocol   uint8
	ServerPort uint16
}
