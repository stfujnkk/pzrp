package test

import (
	"bytes"
	"net"
	"pzrp/pkg/proto"
	"testing"
)

func MsgIsEqual(a, b *proto.Msg) bool {
	if !net.IP.Equal(a.RemoteIP, b.RemoteIP) {
		return false
	}
	if a.RemotePort != b.RemotePort {
		return false
	}
	if a.Action != b.Action {
		return false
	}
	if a.Protocol != b.Protocol {
		return false
	}
	if a.ServerPort != b.ServerPort {
		return false
	}
	if !bytes.Equal(a.Data, b.Data) {
		return false
	}
	return true
}

func TestMsg(t *testing.T) {
	m1 := &proto.Msg{
		RemoteIP:   net.ParseIP("127.0.0.1"),
		RemotePort: 8080,
		Action:     2,
		Protocol:   3,
		ServerPort: 8848,
		Data:       []byte("Hello"),
	}
	p := m1.ToPacket()
	// test 1
	b, err := p.Encode()
	if err != nil {
		t.Errorf("Msg Encode error: %v", err)
	}
	p, _, err = proto.NewPacket(b)
	if err != nil {
		t.Errorf("Msg Decode error: %v", err)
	}
	m2 := p.ToMsg()
	if !MsgIsEqual(m1, m2) {
		t.Error("Unable to recover through decoding after encoding")
	}
	// test 2
	b1 := []byte{0}
	b2 := append(b, b1...)
	p, l, err := proto.NewPacket(b2)
	if err != nil {
		t.Errorf("Msg Decode error: %v", err)
	}
	m3 := p.ToMsg()
	if !MsgIsEqual(m1, m3) {
		t.Error("Unable to recover through decoding after encoding")
	}
	if !bytes.Equal(b1, b2[l:]) {
		t.Error("The length of the processing was not returned correctly")
	}
	// test 3
	b3 := b[:l-1]
	_, l, err = proto.NewPacket(b3)
	if !(l == 0 && err == nil) {
		t.Error("Incomplete data should not be parsed")
	}
}
