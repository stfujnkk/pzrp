package udp

import (
	"context"
	"net"
	pkgErr "pzrp/pkg/errors"
	"pzrp/pkg/proto"
	"pzrp/pkg/utils"
	"time"
)

type UdpServerNode struct {
	ServerPort uint16
	con        *net.UDPConn
	ctx        context.Context
	cancel     context.CancelCauseFunc
}

var _ proto.Node = &UdpServerNode{}

func NewUdpServerNode(con *net.UDPConn, ServerPort uint16, ctx context.Context) *UdpServerNode {
	// TODO add some log
	node := &UdpServerNode{
		con:        con,
		ServerPort: ServerPort,
	}
	node.SetWriteCtx(ctx)
	return node
}

func (node *UdpServerNode) Write(msg proto.Msg) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = utils.NewErr(e)
			node.cancel(err)
		}
	}()
	data := msg.Data
	for len(data) > 0 {
		n, err := node.con.WriteTo(data, &net.UDPAddr{
			IP:   msg.RemoteIP,
			Port: int(msg.RemotePort),
		})
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (node *UdpServerNode) Read() (msg proto.Msg, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = utils.NewErr(e)
			node.cancel(err)
		}
	}()
	buf := make([]byte, 0xffff)
	n, addr, err := node.con.ReadFromUDP(buf)
	if err != nil {
		node.cancel(err)
		return proto.Msg{}, err
	}
	return proto.Msg{
		RemoteIP:   addr.IP,
		RemotePort: uint16(addr.Port),
		Action:     proto.ACTION_SEND_DATA,
		Protocol:   proto.PROTO_UDP,
		ServerPort: node.ServerPort,
		Data:       buf[:n],
	}, nil
}

func (node *UdpServerNode) SetReadCtx(ctx context.Context) {
	node.SetWriteCtx(ctx)
}

func (node *UdpServerNode) SetWriteCtx(ctx context.Context) {
	node.ctx, node.cancel = context.WithCancelCause(ctx)
}

func (node *UdpServerNode) Run() {
	defer func() {
		node.con.Close()
	}()
	<-node.ctx.Done()
}

const SurvivalDuration = 2 * time.Minute

type UdpClientNode struct {
	ServerPort   uint16
	RemoteIP     net.IP
	RemotePort   uint16
	con          *net.UDPConn
	ctx          context.Context
	cancel       context.CancelCauseFunc
	latestTime   time.Time
	sessionTimer *time.Timer
}

func NewUdpClientNode(
	con *net.UDPConn, ServerPort uint16,
	remoteIP net.IP, remotePort uint16,
	ctx context.Context,
) *UdpClientNode {
	node := &UdpClientNode{
		ServerPort: ServerPort,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		con:        con,
	}
	node.SetWriteCtx(ctx)
	return node
}

func (node *UdpClientNode) Write(msg proto.Msg) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = utils.NewErr(e)
			node.cancel(err)
		}
	}()
	data := msg.Data
	for len(data) > 0 {
		n, err := node.con.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	node.latestTime = time.Now()
	return nil
}

func (node *UdpClientNode) Read() (msg proto.Msg, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = utils.NewErr(e)
			node.cancel(err)
		}
	}()
	buf := make([]byte, 0xffff)
	n, err := node.con.Read(buf)
	if err != nil {
		return proto.Msg{}, err
	}
	node.latestTime = time.Now()
	return proto.Msg{
		RemoteIP:   node.RemoteIP,
		RemotePort: node.RemotePort,
		Action:     proto.ACTION_SEND_DATA,
		Protocol:   proto.PROTO_UDP,
		ServerPort: node.ServerPort,
		Data:       buf[:n],
	}, nil
}

func (node *UdpClientNode) SetReadCtx(ctx context.Context) {
	node.SetWriteCtx(ctx)
}

func (node *UdpClientNode) SetWriteCtx(ctx context.Context) {
	node.ctx, node.cancel = context.WithCancelCause(ctx)
}

func (node *UdpClientNode) Run() {
	defer func() {
		node.con.Close()
	}()
	node.latestTime = time.Now()
	node.resetSessionTimer()
	<-node.ctx.Done()
	node.sessionTimer.Stop()
}

func (node *UdpClientNode) resetSessionTimer() {
	now := time.Now()
	idleDuration := now.Sub(node.latestTime)
	if idleDuration+time.Second > SurvivalDuration {
		node.cancel(pkgErr.ErrSessionAging)
		return
	}
	node.sessionTimer = time.AfterFunc(SurvivalDuration-idleDuration, node.resetSessionTimer)
}
