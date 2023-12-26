package v2

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

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

func NewErrWithCtx(
	obj any,
	funcName string,
	args any,
	msg string,
	e any,
) error {
	return fmt.Errorf("%+v %s %+v : %+v : %w", obj, funcName, args, msg, NewErr(e))
}

type Closable interface {
	Close() error
}

type CloseReadAble interface {
	CloseRead() error
}

type CloseWriteAble interface {
	CloseWrite() error
}

type CloseAdapter struct {
	resource any
}

func (ca CloseAdapter) Close() error {
	resource, ok := ca.resource.(Closable)
	if ok {
		return resource.Close()
	}
	return nil
}

func (ca CloseAdapter) CloseRead() error {
	resource, ok := ca.resource.(CloseReadAble)
	if ok {
		return resource.CloseRead()
	}
	return ca.Close()
}

func (ca CloseAdapter) CloseWrite() error {
	resource, ok := ca.resource.(CloseWriteAble)
	if ok {
		return resource.CloseWrite()
	}
	return ca.Close()
}

type PacketNode[T any] interface {
	Read() (T, error)
	Write(T) error
}

type StreamNode[T any] interface {
	Read([]T) (int, error)
	Write([]T) (int, error)
}

// stream ==> packet
type s2pNode[T any, R any] struct {
	stream StreamNode[T]
	pack   func([]T) (R, int, error)
	unpack func(R) ([]T, error)
	data   []T
	buf    []T
	CloseAdapter
}

// 阻塞的
func (node *s2pNode[T, R]) Read() (R, error) {
	for {
		r, n, err := node.pack(node.data)
		if err != nil || n < 0 {
			return r, err
		}
		if n > 0 {
			node.data = node.data[n:]
			return r, nil
		}

		n, err = node.stream.Read(node.buf)
		if err != nil || n <= 0 {
			return r, err
		}
		node.data = append(node.data, node.buf[:n]...)
	}
}

func (node *s2pNode[T, R]) Write(r R) error {
	tt, err := node.unpack(r)
	if err != nil {
		return err
	}
	for len(tt) > 0 {
		n, err := node.stream.Write(tt)
		if err != nil {
			return err
		}
		tt = tt[n:]
	}
	return nil
}

func Stream2Packet[T any, R any](
	stream StreamNode[T],
	pack func([]T) (R, int, error),
	unpack func(R) ([]T, error),
	bufferSize int,
) PacketNode[R] {
	node := &s2pNode[T, R]{
		stream: stream,
		pack:   pack,
		unpack: unpack,
		data:   make([]T, 0),
		buf:    make([]T, bufferSize),
	}
	node.resource = stream
	return node
}

// ChannelNode
type ChannelNode[T any] struct {
	source      PacketNode[T]
	rch         chan T
	wch         chan T
	rErr        error
	wErr        error
	closeHandle CloseAdapter
}

func NewChannelNode[T any](source PacketNode[T], bufferSize int) *ChannelNode[T] {
	node := &ChannelNode[T]{
		source: source,
		rch:    make(chan T, bufferSize),
		wch:    make(chan T, bufferSize),
		closeHandle: CloseAdapter{
			resource: source,
		},
	}
	go node.startRead()
	go node.startWrite()
	return node
}

func (node *ChannelNode[T]) Read() (T, error) {
	msg, ok := <-node.rch
	if ok {
		return msg, nil
	}
	var err error = io.EOF
	if node.rErr != nil {
		err = node.rErr
	}
	return msg, err
}

func (node *ChannelNode[T]) Write(msg T) (err error) {
	defer (func() {
		if e := recover(); e != nil {
			err = io.EOF
			if node.wErr != nil {
				err = node.wErr
			}
		}
	})()
	node.wch <- msg
	return nil
}

func (node *ChannelNode[T]) startRead() {
	defer func() {
		if e := recover(); e != nil {
			node.endRead(NewErrWithCtx(node, "startRead", nil, "", e))
		}
	}()
	for {
		msg, err := node.source.Read()
		if err != nil {
			node.endRead(err)
			break
		}
		node.rch <- msg
	}
}

func (node *ChannelNode[T]) startWrite() {
	defer func() {
		if e := recover(); e != nil {
			node.endWrite(NewErrWithCtx(node, "startWrite", nil, "", e))
		}
	}()
	for msg := range node.wch {
		err := node.source.Write(msg)
		if err != nil {
			node.endWrite(err)
			break
		}
	}
}

// 被动关闭
func (node *ChannelNode[T]) endRead(e error) {
	if e != nil && node.rErr == nil {
		node.rErr = e
	}
	close(node.rch)
}

// 被动关闭
func (node *ChannelNode[T]) endWrite(e error) {
	if e != nil && node.wErr == nil {
		node.wErr = e
	}
	close(node.wch)
}

// 主动关闭
func (node *ChannelNode[T]) CloseRead() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = NewErrWithCtx(node, "CloseRead", nil, "", e)
		}
	}()
	node.endRead(nil)
	return node.closeHandle.CloseRead()
}

// 主动关闭
func (node *ChannelNode[T]) CloseWrite() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = NewErrWithCtx(node, "CloseWrite", nil, "", e)
		}
	}()
	node.endWrite(nil)
	return node.closeHandle.CloseWrite()
}

// 主动关闭
func (node *ChannelNode[T]) Close() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = NewErrWithCtx(node, "Close", nil, "", e)
		}
	}()
	e := node.CloseRead()
	if e != nil && err == nil {
		err = e
	}
	e = node.CloseWrite()
	if e != nil && err == nil {
		err = e
	}
	return err
}

// collect 直接使用goroutine,确保不会泄露goroutine。要有良好的错误处理和隔离。
// 日志信息增强

type RouterErr[K comparable] struct {
	msg string
	err error
	Key K
}

func (e *RouterErr[K]) Error() string {
	return e.msg
}

func (e *RouterErr[K]) Unwrap() error {
	return e.err
}

// mutex sync.Mutex
type Router[T any, K comparable] struct {
	dispatch func(T) (K, error)
	table    *sync.Map
	buf      chan T
	errCh    chan error
}

func NewRouter[T any, K comparable](
	dispatch func(T) (K, error),
	bufferSize int,
) *Router[T, K] {
	router := &Router[T, K]{
		dispatch: dispatch,
		table:    new(sync.Map),
		buf:      make(chan T, bufferSize),
		errCh:    make(chan error, 8),
	}
	return router
}

func (node *Router[T, K]) AddRoute(k K, nextHop PacketNode[T]) {
	node.table.Store(k, nextHop)
	go node.collect(k)
}

func (node *Router[T, K]) RemoveRoute(k K) {
	node.table.Delete(k)
}

func (node *Router[T, K]) collect(k K) {
	defer func() {
		if e := recover(); e != nil {
			node.errCh <- &RouterErr[K]{
				Key: k,
				msg: fmt.Sprintf("%+v collect %+v : %+v", node, []any{k}, e),
				err: NewErr(e),
			}
		}
	}()
	value, ok := node.table.Load(k)
	if !ok {
		panic(fmt.Errorf("cannot find %+v in route table", k))
	}
	nextHop := value.(PacketNode[T])
	for {
		msg, err := nextHop.Read()
		if err != nil {
			panic(err)
		}
		node.buf <- msg
	}
}

func (node *Router[T, K]) Read() (T, error) {
	select {
	case msg, ok := <-node.buf:
		if !ok {
			return msg, io.EOF
		}
		return msg, nil
	case err := <-node.errCh:
		return *new(T), err
	}
}

func (node *Router[T, K]) Write(msg T) error {
	k, err := node.dispatch(msg)
	if err != nil {
		return err
	}
	nextHop, ok := node.table.Load(k)
	if !ok {
		return fmt.Errorf("%+v not found: %+v", k, msg)
	}
	return nextHop.(PacketNode[T]).Write(msg)
}

func (node *Router[T, K]) Close() (err error) {
	close(node.buf)
	node.table.Range(func(key, value any) bool {
		defer func() {
			if e := recover(); e != nil {
				err = NewErrWithCtx(node, "Close", []any{key, value}, "", e)
			}
		}()
		e := CloseAdapter{
			resource: value,
		}.Close()
		if err == nil && e != nil {
			err = e
		}
		return true
	})
	return err
}

// Edge(p,s)
// 上层调用Read，Write
// go
