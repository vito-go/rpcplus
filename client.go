package gorpc

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

type Client struct {
	codec    CodecClient
	seq      atomic.Uint64
	reqMutex sync.Mutex

	mutex    sync.Mutex
	waitCall map[uint64]*Call
}

func (client *Client) input() {
	defer func() {
		_ = client.codec.Close()
		client.mutex.Lock()
		for u, call := range client.waitCall {
			close(call.Done)
			delete(client.waitCall, u)
		}
		client.mutex.Unlock()
	}()
	for {
		response, err := client.codec.ReadResponseHeader()
		if err != nil {
			log.Println("client read response header error:", err)
			return
		}
		seq := response.Seq
		client.mutex.Lock()
		call, ok := client.waitCall[seq]
		client.mutex.Unlock()
		if !ok {
			log.Println(seq, "nil call")
			// should not happen?
			continue
		}
		err = client.codec.ReadResponseBody(call.Reply)
		call.Error = err
		if response.Code != 200 {
			call.Error = errors.New(response.Message)
		}
		call.Done <- call
	}
}

func (client *Client) Close() error {
	err := client.codec.Close()
	if err != nil {
		return err
	}
	return nil
}
func NewClient(conn net.Conn) *Client {
	codec := newGobClientCodec(conn)
	client := Client{
		codec:    codec,
		seq:      atomic.Uint64{},
		reqMutex: sync.Mutex{},
		waitCall: make(map[uint64]*Call, 1024),
	}
	go client.input()
	return &client
}

// Call represents an active RPC.
type Call struct {
	Reply interface{} // The reply from the function (*struct).
	Error error       // After completion, the error status.
	Done  chan *Call  // Receives *Call when Go is complete.
}

func (client *Client) Call(ctx context.Context, serviceMethod string, reply any, args ...interface{}) (err error) {
	return client.call(ctx, serviceMethod, reply, args...)
}
func (client *Client) CallVoid(ctx context.Context, serviceMethod string, args ...interface{}) (err error) {
	return client.call(ctx, serviceMethod, nil, args...)
}
func (client *Client) call(ctx context.Context, serviceMethod string, reply any, args ...interface{}) (err error) {
	doneCall, err := client.Go(ctx, reply, serviceMethod, args...)
	if err != nil {
		_ = client.codec.Close()
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case _, ok := <-doneCall.Done:
		if !ok {
			return io.ErrClosedPipe
		}
	}
	if doneCall.Error != nil {
		return doneCall.Error
	}
	return nil
}

func (client *Client) Go(ctx context.Context, reply any, serviceMethod string, args ...interface{}) (*Call, error) {
	seq := client.seq.Add(1)
	var noReply bool
	if reply == nil {
		noReply = true
	}
	request := &Request{
		NoReply:       noReply,
		ServiceMethod: serviceMethod,
		ArgsNum:       len(args),
		Seq:           seq,
	}
	done := make(chan *Call, 1)
	call := &Call{
		Reply: reply,
		Error: nil,
		Done:  done,
	}
	client.mutex.Lock()
	client.waitCall[seq] = call
	client.mutex.Unlock()
	client.reqMutex.Lock()
	defer client.reqMutex.Unlock()
	err := client.codec.WriteRequest(request, args...)
	if err != nil {
		client.mutex.Lock()
		delete(client.waitCall, seq)
		client.mutex.Unlock()
		return nil, err
	}
	return call, nil
}
