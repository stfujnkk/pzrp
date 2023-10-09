package proto

import (
	"context"
	"net"
)

// 协议
const (
	PROTO_NUL = uint8(iota)
	PROTO_TCP = uint8(iota)
	PROTO_UDP = uint8(iota)
)

var StrToProto = map[string]uint8{
	"tcp": PROTO_TCP,
	"udp": PROTO_UDP,
}

var ProtoToStr = map[uint8]string{
	PROTO_TCP: "tcp",
	PROTO_UDP: "udp",
}

// 指令
const (
	ACTION_SEND_DATA   = uint8(iota)
	ACTION_CLOSE_READ  = uint8(iota)
	ACTION_CLOSE_WRITE = uint8(iota)
	ACTION_CLOSE_ALL   = ACTION_CLOSE_READ | ACTION_CLOSE_WRITE
	ACTION_SET_CONFIG  = uint8(iota)
)

type Node interface {
	Read() (Msg, error)
	Write(Msg) error
	SetReadCtx(context.Context)
	SetWriteCtx(context.Context)
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
	Data []byte
}

type PacketHead struct {
	RemoteIP   [4]byte
	RemotePort uint16
	Action     uint8
	Protocol   uint8
	ServerPort uint16
}
