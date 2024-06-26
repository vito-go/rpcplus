// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vito-go/rpcplus"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

type Arith int

type ArithAddResp struct {
	Id     any   `json:"id"`
	Result Reply `json:"result"`
	Error  any   `json:"error"`
}

func GetAge(ctx context.Context, userName string, classId int) (int, error) {
	return 31, nil
}
func (t *Arith) Add(ctx context.Context, args *Args) (*Reply, error) {
	reply := new(Reply)
	reply.C = args.A + args.B
	return reply, nil
}

func (t *Arith) Year(ctx context.Context) (*Reply, error) {
	reply := new(Reply)

	reply.C = 2024
	return reply, nil
}

func (t *Arith) Mul(ctx context.Context, args *Args) (*Reply, error) {
	reply := new(Reply)

	reply.C = args.A * args.B
	return reply, nil
}

func (t *Arith) Div(ctx context.Context, args *Args) (*Reply, error) {
	reply := new(Reply)

	if args.B == 0 {
		return nil, errors.New("divide by zero")
	}
	reply.C = args.A / args.B
	return reply, nil
}

func (t *Arith) Error(ctx context.Context, args *Args) (*Reply, error) {
	panic("ERROR")
}

type BuiltinTypes struct{}

func (BuiltinTypes) Map(ctx context.Context, i int) (*map[int]int, error) {
	reply := new(map[int]int)
	*reply = make(map[int]int)
	(*reply)[i] = i
	return reply, nil
}

func (BuiltinTypes) Slice(ctx context.Context, i int) (*[]int, error) {
	reply := new([]int)
	*reply = make([]int, 0)
	*reply = append(*reply, i)
	return reply, nil
}

func (BuiltinTypes) Array(ctx context.Context, i int) (*[1]int, error) {
	reply := new([1]int)
	(*reply)[0] = i
	return reply, nil
}

func init() {
	var err error
	err = rpcplus.RegisterRecv(new(Arith))
	if err != nil {
		panic(err)
	}
	err = rpcplus.RegisterRecv(BuiltinTypes{})
	if err != nil {
		panic(err)
	}
	err = rpcplus.RegisterFunc("GetAge", GetAge)
	if err != nil {
		panic(err)
	}
}

func TestServerNoParams(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	go ServeConn(srv)
	dec := json.NewDecoder(cli)

	fmt.Fprintf(cli, `{"method": "Arith.Add", "id": "123"}`)
	var resp ArithAddResp
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("Decode after no params: %s", err)
	}
	if resp.Error == nil {
		t.Fatalf("Expected error, got nil")
	}
}

func TestServerEmptyMessage(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	go ServeConn(srv)
	dec := json.NewDecoder(cli)

	fmt.Fprintf(cli, "{}")
	var resp ArithAddResp
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("Decode after empty: %s", err)
	}
	if resp.Error == nil {
		t.Fatalf("Expected error, got nil")
	}
}

func TestServer(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	go ServeConn(srv)
	dec := json.NewDecoder(cli)

	// Send hand-coded requests to server, parse responses.
	for i := 0; i < 10; i++ {
		fmt.Fprintf(cli, `{"method": "Arith.Add", "id": "\u%04d", "params": [{"A": %d, "B": %d}]}`, i, i, i+1)
		var resp ArithAddResp
		err := dec.Decode(&resp)
		if err != nil {
			t.Fatalf("Decode: %s", err)
		}
		if resp.Error != nil {
			t.Fatalf("resp.Error: %s", resp.Error)
		}
		if resp.Id.(string) != string(rune(i)) {
			t.Fatalf("resp: bad id %q want %q", resp.Id.(string), string(rune(i)))
		}
		if resp.Result.C != 2*i+1 {
			t.Fatalf("resp: bad result: %d+%d=%d", i, i+1, resp.Result.C)
		}
	}
}

func TestClient(t *testing.T) {
	// Assume server is okay (TestServer is above).
	// Test client against server.
	cli, srv := net.Pipe()
	go ServeConn(srv)

	client := NewClient(cli)
	defer client.Close()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err := client.Call("Arith.Add", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: got %d expected %d", reply.C, args.A+args.B)
	}

	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call("Arith.Mul", reply, args)
	if err != nil {
		t.Errorf("Mul: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A*args.B {
		t.Errorf("Mul: got %d expected %d", reply.C, args.A*args.B)
	}
	err = client.Call("Arith.Year", reply)
	if err != nil {
		t.Errorf("Mul: expected no error but got string %q", err.Error())
	}
	if reply.C != 2024 {
		t.Errorf("Mul: got %d expected %d", reply.C, 2024)
	}
	var age int
	err = client.Call("GetAge", &age, "Vito", 11)
	if err != nil {
		t.Errorf("GetAge: expected no error but got string %q", err.Error())
	}
	if age != 31 {
		t.Errorf("GetAge: got %d expected %d", reply.C, 31)
	}

	// Out of order.
	args = &Args{7, 8}
	mulReply := new(Reply)
	mulCall := client.Go("Arith.Mul", mulReply, nil, args)
	addReply := new(Reply)
	addCall := client.Go("Arith.Add", addReply, nil, args)
	addCall = <-addCall.Done
	if addCall.Error != nil {
		t.Errorf("Add: expected no error but got string %q", addCall.Error.Error())
	}
	if addReply.C != args.A+args.B {
		t.Errorf("Add: got %d expected %d", addReply.C, args.A+args.B)
	}

	mulCall = <-mulCall.Done
	if mulCall.Error != nil {
		t.Errorf("Mul: expected no error but got string %q", mulCall.Error.Error())
	}
	if mulReply.C != args.A*args.B {
		t.Errorf("Mul: got %d expected %d", mulReply.C, args.A*args.B)
	}

	// Error test
	args = &Args{7, 0}
	reply = new(Reply)
	err = client.Call("Arith.Div", reply, args)
	// expect an error: zero divide
	if err == nil {
		t.Error("Div: expected error")
	} else if err.Error() != "divide by zero" {
		t.Error("Div: expected divide by zero error; got", err)
	}
}

func TestBuiltinTypes(t *testing.T) {
	cli, srv := net.Pipe()
	go ServeConn(srv)

	client := NewClient(cli)
	defer client.Close()

	// Map
	arg := 7
	replyMap := map[int]int{}
	err := client.Call("BuiltinTypes.Map", &replyMap, arg)
	if err != nil {
		t.Errorf("Map: expected no error but got string %q", err.Error())
	}
	if replyMap[arg] != arg {
		t.Errorf("Map: expected %d got %d", arg, replyMap[arg])
	}

	// Slice
	var replySlice []int
	err = client.Call("BuiltinTypes.Slice", &replySlice, arg)
	if err != nil {
		t.Errorf("Slice: expected no error but got string %q", err.Error())
	}
	if e := []int{arg}; !reflect.DeepEqual(replySlice, e) {
		t.Errorf("Slice: expected %v got %v", e, replySlice)
	}

	// Array
	replyArray := [1]int{}
	err = client.Call("BuiltinTypes.Array", &replyArray, arg)
	if err != nil {
		t.Errorf("Array: expected no error but got string %q", err.Error())
	}
	if e := [1]int{arg}; !reflect.DeepEqual(replyArray, e) {
		t.Errorf("Array: expected %v got %v", e, replyArray)
	}
}

func TestMalformedInput(t *testing.T) {
	cli, srv := net.Pipe()
	go cli.Write([]byte(`{id:1}`)) // invalid json
	ServeConn(srv)                 // must return, not loop
}

func TestMalformedOutput(t *testing.T) {
	cli, srv := net.Pipe()
	go srv.Write([]byte(`{"id":0,"result":null,"error":null}`))
	go io.ReadAll(srv)

	client := NewClient(cli)
	defer client.Close()

	args := &Args{7, 8}
	reply := new(Reply)
	err := client.Call("Arith.Add", reply, args)
	if err == nil {
		t.Error("expected error")
	}
}

func TestServerErrorHasNullResult(t *testing.T) {
	var out strings.Builder
	sc := NewServerCodec(struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: strings.NewReader(`{"method": "Arith.Add", "id": "123", "params": []}`),
		Writer: &out,
		Closer: io.NopCloser(nil),
	})
	r := new(rpcplus.Request)
	if err := sc.ReadRequestHeader(r); err != nil {
		t.Fatal(err)
	}
	const valueText = "the value we don't want to see"
	const errorText = "some error"
	err := sc.WriteResponse(&rpcplus.Response{
		ServiceMethod: "Method",
		Seq:           1,
		Error:         errorText,
	}, valueText)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), errorText) {
		t.Fatalf("Response didn't contain expected error %q: %s", errorText, &out)
	}
	if strings.Contains(out.String(), valueText) {
		t.Errorf("Response contains both an error and value: %s", &out)
	}
}

func TestUnexpectedError(t *testing.T) {
	cli, srv := myPipe()
	go cli.PipeWriter.CloseWithError(errors.New("unexpected error")) // reader will get this error
	ServeConn(srv)                                                   // must return, not loop
}

// Copied from package net.
func myPipe() (*pipe, *pipe) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	return &pipe{r1, w2}, &pipe{r2, w1}
}

type pipe struct {
	*io.PipeReader
	*io.PipeWriter
}

type pipeAddr int

func (pipeAddr) Network() string {
	return "pipe"
}

func (pipeAddr) String() string {
	return "pipe"
}

func (p *pipe) Close() error {
	err := p.PipeReader.Close()
	err1 := p.PipeWriter.Close()
	if err == nil {
		err = err1
	}
	return err
}

func (p *pipe) LocalAddr() net.Addr {
	return pipeAddr(0)
}

func (p *pipe) RemoteAddr() net.Addr {
	return pipeAddr(0)
}

func (p *pipe) SetTimeout(nsec int64) error {
	return errors.New("net.Pipe does not support timeouts")
}

func (p *pipe) SetReadTimeout(nsec int64) error {
	return errors.New("net.Pipe does not support timeouts")
}

func (p *pipe) SetWriteTimeout(nsec int64) error {
	return errors.New("net.Pipe does not support timeouts")
}
