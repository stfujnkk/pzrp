package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var pkgOrder = binary.BigEndian

var pkgHeadOffset = 2 + binary.Size(PacketHead{})

func NewPacket(data []byte) (*Packet, int, error) {
	if len(data) < pkgHeadOffset {
		// 数据包不完整
		return nil, 0, nil
	}
	var (
		buf = bytes.NewBuffer(data)
		pkg = Packet{}
	)
	// length
	err := binary.Read(buf, pkgOrder, &pkg.Length)
	if err != nil {
		return nil, 0, err
	}
	size := int(pkg.Length) + pkgHeadOffset
	if len(data) < size {
		// 数据包不完整
		return nil, 0, nil
	}
	// head
	err = binary.Read(buf, pkgOrder, &pkg.PacketHead)
	if err != nil {
		return nil, 0, err
	}
	// data
	pkg.Data = data[pkgHeadOffset:size]
	return &pkg, size, nil
}

func (pkg *Packet) Encode() ([]byte, error) {
	var (
		buf = new(bytes.Buffer)
		l   = len(pkg.Data)
	)
	if l > 0xffff {
		return nil, errors.New("data length cannot exceed 0xffff")
	}
	// length
	err := binary.Write(buf, pkgOrder, uint16(l))
	if err != nil {
		return nil, err
	}
	// head
	err = binary.Write(buf, pkgOrder, pkg.PacketHead)
	if err != nil {
		return nil, err
	}
	// data
	err = binary.Write(buf, pkgOrder, pkg.Data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (pkg *Packet) ToMsg() *Msg {
	return &Msg{
		RemoteIP:   pkg.RemoteIP[:],
		RemotePort: pkg.RemotePort,
		Action:     pkg.Action,
		Protocol:   pkg.Protocol,
		ServerPort: pkg.ServerPort,
		Data:       pkg.Data,
	}
}

func (msg *Msg) ToPacket() *Packet {

	return &Packet{
		PacketHead: PacketHead{
			RemoteIP:   ([4]byte)(msg.RemoteIP.To4()),
			RemotePort: uint16(msg.RemotePort),
			Action:     uint8(msg.Action),
			Protocol:   uint8(msg.Protocol),
			ServerPort: uint16(msg.ServerPort),
		},
		Length: uint16(len(msg.Data)),
		Data:   msg.Data,
	}
}

func IsCloseMsg(msg Msg) bool {
	return msg.Action == ACTION_CLOSE_READ || msg.Action == ACTION_CLOSE_WRITE || msg.Action == ACTION_CLOSE_ALL
}
