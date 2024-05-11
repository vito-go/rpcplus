// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpcplus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	newServer                 *Server
	serverAddr, newServerAddr string
	httpServerAddr            string
	once, newOnce, httpOnce   sync.Once
)

const (
	newHttpPath = "/foo"
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

type Arith int

// Some of Arith's methods have value args, some have pointer args. That's deliberate.

func (t *Arith) Add(ctx context.Context, args *Args) (*Reply, error) {
	reply := Reply{C: args.A + args.B}
	return &reply, nil
}

func (t *Arith) Mul(ctx context.Context, args *Args) (reply *Reply, err error) {
	reply = new(Reply)
	reply.C = args.A * args.B
	return reply, nil
}

func (t *Arith) Div(ctx context.Context, args Args) (*Reply, error) {
	reply := new(Reply)
	if args.B == 0 {
		return nil, errors.New("divide by zero")
	}
	reply.C = args.A / args.B
	return reply, nil
}

func (t *Arith) String(ctx context.Context, args *Args) (*string, error) {
	reply := fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	return &reply, nil
}

func (t *Arith) Scan(ctx context.Context, args string) (reply *Reply, err error) {
	reply = new(Reply)
	_, err = fmt.Sscan(args, &reply.C)
	return
}

func (t *Arith) Error(ctx context.Context, args *Args) (*Reply, error) {
	panic("ERROR")
}

func (t *Arith) SleepMilli(ctx context.Context, args Args) (*Reply, error) {
	time.Sleep(time.Duration(args.A) * time.Millisecond)
	return new(Reply), nil
}

type hidden int

func (t *hidden) Exported(ctx context.Context, args Args) (*Reply, error) {
	reply := new(Reply)
	reply.C = args.A + args.B
	return reply, nil
}

type Embed struct {
	hidden
}

type BuiltinTypes struct{}

func (BuiltinTypes) Map(ctx context.Context, args *Args) (reply *map[int]int, err error) {
	reply = new(map[int]int)
	*reply = make(map[int]int)
	(*reply)[args.A] = args.B
	return reply, nil
}

func (BuiltinTypes) Slice(ctx context.Context, args *Args) (reply *[]int, err error) {
	reply = new([]int)
	*reply = append(*reply, args.A, args.B)
	return
}

func (BuiltinTypes) Array(ctx context.Context, args *Args) (reply *[2]int, err error) {
	reply = new([2]int)
	(*reply)[0] = args.A
	(*reply)[1] = args.B
	return
}

func listenTCP() (net.Listener, string) {
	l, err := net.Listen("tcp", "127.0.0.1:0") // any available address
	if err != nil {
		log.Fatalf("net.Listen tcp :0: %v", err)
	}
	return l, l.Addr().String()
}

func startServer() {
	Register(new(Arith))
	Register(new(Embed))
	RegisterName("net.rpcplus.Arith", new(Arith))
	err := Register(BuiltinTypes{})
	if err != nil {
		panic(err)
	}

	var l net.Listener
	l, serverAddr = listenTCP()
	log.Println("Test RPC server listening on", serverAddr)
	go Accept(l)

	HandleHTTP()
	httpOnce.Do(startHttpServer)
}

func startNewServer() {
	newServer = NewServer()
	newServer.RegisterRecv(new(Arith))
	newServer.RegisterRecv(new(Embed))
	newServer.RegisterRecvWithName("net.rpcplus.Arith", new(Arith))
	newServer.RegisterRecvWithName("newServer.Arith", new(Arith))

	var l net.Listener
	l, newServerAddr = listenTCP()
	log.Println("NewServer test RPC server listening on", newServerAddr)
	go newServer.Accept(l)

	newServer.HandleHTTP(newHttpPath, "/bar")
	httpOnce.Do(startHttpServer)
}

func startHttpServer() {
	server := httptest.NewServer(nil)
	httpServerAddr = server.Listener.Addr().String()
	log.Println("Test HTTP RPC server listening on", httpServerAddr)
}

func TestRPC(t *testing.T) {
	once.Do(startServer)
	testRPC(t, serverAddr)
	newOnce.Do(startNewServer)
	testRPC(t, newServerAddr)
	testNewServerRPC(t, newServerAddr)
}

func testRPC(t *testing.T, addr string) {
	client, err := Dial("tcp", addr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()

	// Synchronous calls

	args := &Args{7, 8}
	reply := new(Reply)
	err = client.Call("Arith.Add", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}

	// Methods exported from unexported embedded structs
	args = &Args{7, 0}

	reply = new(Reply)
	err = client.Call("Embed.Exported", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}

	// Nonexistent method
	args = &Args{7, 0}

	reply = new(Reply)
	err = client.Call("Arith.BadOperation", reply, args)
	// expect an error
	if err == nil {
		t.Error("BadOperation: expected error")
	} else if !strings.HasPrefix(err.Error(), "rpcplus: can't find method") {
		t.Errorf("BadOperation: expected can't find method error; got %q", err)
	}

	// Unknown service
	args = &Args{7, 8}

	reply = new(Reply)
	err = client.Call("Arith.Unknown", reply, args)
	if err == nil {
		t.Error("expected error calling unknown service")
	} else if !strings.Contains(err.Error(), "method") {
		t.Error("expected error about method; got", err)
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
		t.Errorf("Add: expected %d got %d", addReply.C, args.A+args.B)
	}

	mulCall = <-mulCall.Done
	if mulCall.Error != nil {
		t.Errorf("Mul: expected no error but got string %q", mulCall.Error.Error())
	}
	if mulReply.C != args.A*args.B {
		t.Errorf("Mul: expected %d got %d", mulReply.C, args.A*args.B)
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

	// Bad type.
	reply = new(Reply)
	err = client.Call("Arith.Add", reply, reply) // reply,args would be the correct thing to use
	if err == nil {
		t.Error("expected error calling Arith.Add with wrong arg type")
	} else if !strings.Contains(err.Error(), "type") {
		t.Error("expected error about type; got", err)
	}

	// Non-struct argument
	const Val = 12345
	str := fmt.Sprint(Val)
	reply = new(Reply)
	err = client.Call("Arith.Scan", reply, &str)
	if err != nil {
		t.Errorf("Scan: expected no error but got string %q", err.Error())
	} else if reply.C != Val {
		t.Errorf("Scan: expected %d got %d", Val, reply.C)
	}

	// Non-struct reply
	args = &Args{27, 35}
	str = ""
	err = client.Call("Arith.String", &str, args)
	if err != nil {
		t.Errorf("String: expected no error but got string %q", err.Error())
	}
	expect := fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	if str != expect {
		t.Errorf("String: expected %s got %s", expect, str)
	}

	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call("Arith.Mul", reply, args)
	if err != nil {
		t.Errorf("Mul: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A*args.B {
		t.Errorf("Mul: expected %d got %d", reply.C, args.A*args.B)
	}

	// ServiceName contain "." character
	args = &Args{7, 8}
	reply = new(Reply)
	err = client.Call("net.rpcplus.Arith.Add", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}
}

func testNewServerRPC(t *testing.T, addr string) {
	client, err := Dial("tcp", addr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err = client.Call("newServer.Arith.Add", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}
}

func TestHTTP(t *testing.T) {
	once.Do(startServer)
	testHTTPRPC(t, "")
	newOnce.Do(startNewServer)
	testHTTPRPC(t, newHttpPath)
}

func testHTTPRPC(t *testing.T, path string) {
	var client *Client
	var err error
	if path == "" {
		client, err = DialHTTP("tcp", httpServerAddr)
	} else {
		client, err = DialHTTPPath("tcp", httpServerAddr, path)
	}
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()

	// Synchronous calls
	args := &Args{7, 8}
	reply := new(Reply)
	err = client.Call("Arith.Add", reply, args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}
}

func TestBuiltinTypes(t *testing.T) {
	once.Do(startServer)

	client, err := DialHTTP("tcp", httpServerAddr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()

	// Map
	args := &Args{7, 8}
	replyMap := map[int]int{}
	err = client.Call("BuiltinTypes.Map", &replyMap, args)
	if err != nil {
		t.Errorf("Map: expected no error but got string %q", err.Error())
	}
	if replyMap[args.A] != args.B {
		t.Errorf("Map: expected %d got %d", args.B, replyMap[args.A])
	}

	// Slice
	args = &Args{7, 8}
	replySlice := []int{}
	err = client.Call("BuiltinTypes.Slice", &replySlice, args)
	if err != nil {
		t.Errorf("Slice: expected no error but got string %q", err.Error())
	}
	if e := []int{args.A, args.B}; !reflect.DeepEqual(replySlice, e) {
		t.Errorf("Slice: expected %v got %v", e, replySlice)
	}

	// Array
	args = &Args{7, 8}
	replyArray := [2]int{}
	err = client.Call("BuiltinTypes.Array", &replyArray, args)
	if err != nil {
		t.Errorf("Array: expected no error but got string %q", err.Error())
	}
	if e := [2]int{args.A, args.B}; !reflect.DeepEqual(replyArray, e) {
		t.Errorf("Array: expected %v got %v", e, replyArray)
	}
}

// CodecEmulator provides a client-like api and a ServerCodec interface.
// Can be used to test ServeRequest.
type CodecEmulator struct {
	server        *Server
	serviceMethod string
	args          []*Args
	reply         *Reply
	err           error
}

func (codec *CodecEmulator) Call(serviceMethod string, args ...*Args) (*Reply, error) {
	reply := new(Reply)
	codec.serviceMethod = serviceMethod
	codec.args = args
	codec.reply = reply
	codec.err = nil
	var serverError error
	if codec.server == nil {
		serverError = ServeRequest(codec)
	} else {
		serverError = codec.server.ServeRequest(codec)
	}
	if codec.err == nil && serverError != nil {
		codec.err = serverError
	}
	return reply, codec.err
}

func (codec *CodecEmulator) ReadRequestHeader(req *Request) error {
	req.ServiceMethod = codec.serviceMethod
	req.Seq = 0
	req.ArgNum = len(codec.args)
	return nil
}

func (codec *CodecEmulator) ReadRequestBody(argvs ...any) error {
	if codec.args == nil {
		return io.ErrUnexpectedEOF
	}
	for i := 0; i < len(argvs); i++ {
		*(argvs[i].(*Args)) = *(codec.args[i])

	}
	//for _, argv := range argvs {
	//	*(argv.(*Args)) = *codec.args
	//}
	return nil
}

func (codec *CodecEmulator) WriteResponse(resp *Response, reply any) error {
	if resp.Error != "" {
		codec.err = errors.New(resp.Error)
	} else {
		*codec.reply = *(reply.(*Reply))
	}
	return nil
}

func (codec *CodecEmulator) Close() error {
	return nil
}

func TestServeRequest(t *testing.T) {
	once.Do(startServer)
	testServeRequest(t, nil)
	newOnce.Do(startNewServer)
	testServeRequest(t, newServer)
}

func testServeRequest(t *testing.T, server *Server) {
	client := CodecEmulator{server: server}
	defer client.Close()

	args := &Args{7, 8}
	reply := new(Reply)
	var err error
	reply, err = client.Call("Arith.Add", args)
	if err != nil {
		t.Errorf("Add: expected no error but got string %q", err.Error())
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
	}

	reply, err = client.Call("Arith.Add")
	if err == nil {
		t.Errorf("expected error calling Arith.Add with no arg")
	}
}

type local struct{}

func NoContext(args *Args) (Reply, error) {
	return Reply{}, nil
}

func OneReturnButNotError(ctx context.Context, args *local) *Reply {
	return new(Reply)
}

func OneReturn2thNotError(ctx context.Context, args *Args) (*local, bool) {
	return new(local), false
}

func MoreThan2Return(ctx context.Context, args *Args) (*Reply, bool, error) {
	return new(Reply), false, nil
}

// Check that registration handles lots of bad methods and a type with no suitable methods.
func TestRegistrationError(t *testing.T) {
	err := Register(NoContext)
	if err == nil {
		t.Error("expected error registering NoContext")
	}
	err = Register(OneReturnButNotError)
	if err == nil {
		t.Error("expected error registering ArgNotPublic")
	}
	err = Register(OneReturn2thNotError)
	if err == nil {
		t.Error("expected error registering OneReturn2thNotError")
	}
	err = Register(MoreThan2Return)
	if err == nil {
		t.Error("expected error registering MoreThan2Return")
	}

}

type WriteFailCodec int

func (WriteFailCodec) WriteRequest(*Request, ...any) error {
	// the panic caused by this error used to not unlock a lock.
	return errors.New("fail")
}

func (WriteFailCodec) ReadResponseHeader(*Response) error {
	select {}
}

func (WriteFailCodec) ReadResponseBody(any) error {
	select {}
}

func (WriteFailCodec) Close() error {
	return nil
}

func TestSendDeadlock(t *testing.T) {
	client := NewClientWithCodec(WriteFailCodec(0))
	defer client.Close()

	done := make(chan bool)
	go func() {
		testSendDeadlock(client)
		testSendDeadlock(client)
		done <- true
	}()
	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock")
	}
}

func testSendDeadlock(client *Client) {
	defer func() {
		recover()
	}()
	args := &Args{7, 8}
	reply := new(Reply)
	client.Call("Arith.Add", reply, args)
}

func dialDirect() (*Client, error) {
	return Dial("tcp", serverAddr)
}

func dialHTTP() (*Client, error) {
	return DialHTTP("tcp", httpServerAddr)
}

func countMallocs(dial func() (*Client, error), t *testing.T) float64 {
	once.Do(startServer)
	client, err := dial()
	if err != nil {
		t.Fatal("error dialing", err)
	}
	defer client.Close()

	args := &Args{7, 8}
	reply := new(Reply)
	return testing.AllocsPerRun(100, func() {
		err := client.Call("Arith.Add", reply, args)
		if err != nil {
			t.Errorf("Add: expected no error but got string %q", err.Error())
		}
		if reply.C != args.A+args.B {
			t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
		}
	})
}

func TestCountMallocs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping malloc count in short mode")
	}
	if runtime.GOMAXPROCS(0) > 1 {
		t.Skip("skipping; GOMAXPROCS>1")
	}
	fmt.Printf("mallocs per rpcplus round trip: %v\n", countMallocs(dialDirect, t))
}

func TestCountMallocsOverHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping malloc count in short mode")
	}
	if runtime.GOMAXPROCS(0) > 1 {
		t.Skip("skipping; GOMAXPROCS>1")
	}
	fmt.Printf("mallocs per HTTP rpcplus round trip: %v\n", countMallocs(dialHTTP, t))
}

type writeCrasher struct {
	done chan bool
}

func (writeCrasher) Close() error {
	return nil
}

func (w *writeCrasher) Read(p []byte) (int, error) {
	<-w.done
	return 0, io.EOF
}

func (writeCrasher) Write(p []byte) (int, error) {
	return 0, errors.New("fake write failure")
}

func TestClientWriteError(t *testing.T) {
	w := &writeCrasher{done: make(chan bool)}
	c := NewClient(w)
	defer c.Close()

	res := false
	err := c.Call("foo", 1, &res)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "fake write failure" {
		t.Error("unexpected value of error:", err)
	}
	w.done <- true
}

func TestTCPClose(t *testing.T) {
	once.Do(startServer)

	client, err := dialHTTP()
	if err != nil {
		t.Fatalf("dialing: %v", err)
	}
	defer client.Close()

	args := Args{17, 8}
	var reply Reply
	err = client.Call("Arith.Mul", &reply, args)
	if err != nil {
		t.Fatal("arith error:", err)
	}
	t.Logf("Arith: %d*%d=%d\n", args.A, args.B, reply)
	if reply.C != args.A*args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A*args.B)
	}
}

func TestErrorAfterClientClose(t *testing.T) {
	once.Do(startServer)

	client, err := dialHTTP()
	if err != nil {
		t.Fatalf("dialing: %v", err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal("close error:", err)
	}
	err = client.Call("Arith.Add", &Args{7, 9}, new(Reply))
	if err != ErrShutdown {
		t.Errorf("Forever: expected ErrShutdown got %v", err)
	}
}

// Tests the fix to issue 11221. Without the fix, this loops forever or crashes.
func TestAcceptExitAfterListenerClose(t *testing.T) {
	newServer := NewServer()
	newServer.Register(new(Arith))
	newServer.RegisterName("net.rpcplus.Arith", new(Arith))
	newServer.RegisterName("newServer.Arith", new(Arith))

	var l net.Listener
	l, _ = listenTCP()
	l.Close()
	newServer.Accept(l)
}

func TestShutdown(t *testing.T) {
	var l net.Listener
	l, _ = listenTCP()
	ch := make(chan net.Conn, 1)
	go func() {
		defer l.Close()
		c, err := l.Accept()
		if err != nil {
			t.Error(err)
		}
		ch <- c
	}()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c1 := <-ch
	if c1 == nil {
		t.Fatal(err)
	}

	newServer := NewServer()
	newServer.Register(new(Arith))
	go newServer.ServeConn(c1)

	args := &Args{7, 8}
	reply := new(Reply)
	client := NewClient(c)
	err = client.Call("Arith.Add", reply, args)
	if err != nil {
		t.Fatal(err)
	}

	// On an unloaded system 10ms is usually enough to fail 100% of the time
	// with a broken server. On a loaded system, a broken server might incorrectly
	// be reported as passing, but we're OK with that kind of flakiness.
	// If the code is correct, this test will never fail, regardless of timeout.
	args.A = 10 // 10 ms
	done := make(chan *Call, 1)
	call := client.Go("Arith.SleepMilli", reply, done, args)
	c.(*net.TCPConn).CloseWrite()
	<-done
	if call.Error != nil {
		t.Fatal(call.Error)
	}
}

func benchmarkEndToEnd(dial func() (*Client, error), b *testing.B) {
	once.Do(startServer)
	client, err := dial()
	if err != nil {
		b.Fatal("error dialing:", err)
	}
	defer client.Close()

	// Synchronous calls
	args := &Args{7, 8}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		reply := new(Reply)
		for pb.Next() {
			err := client.Call("Arith.Add", reply, args)
			if err != nil {
				b.Fatalf("rpcplus error: Add: expected no error but got string %q", err.Error())
			}
			if reply.C != args.A+args.B {
				b.Fatalf("rpcplus error: Add: expected %d got %d", reply.C, args.A+args.B)
			}
		}
	})
}

func benchmarkEndToEndAsync(dial func() (*Client, error), b *testing.B) {
	if b.N == 0 {
		return
	}
	const MaxConcurrentCalls = 100
	once.Do(startServer)
	client, err := dial()
	if err != nil {
		b.Fatal("error dialing:", err)
	}
	defer client.Close()

	// Asynchronous calls
	args := &Args{7, 8}
	argss := []any{args}
	procs := 4 * runtime.GOMAXPROCS(-1)
	send := int32(b.N)
	recv := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	gate := make(chan bool, MaxConcurrentCalls)
	res := make(chan *Call, MaxConcurrentCalls)
	b.ResetTimer()

	for p := 0; p < procs; p++ {
		go func() {
			for atomic.AddInt32(&send, -1) >= 0 {
				gate <- true
				reply := new(Reply)
				client.Go("Arith.Add", reply, res, argss...)
			}
		}()
		go func() {
			for call := range res {
				A := call.Args[0].(*Args).A
				B := call.Args[0].(*Args).B
				C := call.Reply.(*Reply).C
				if A+B != C {
					b.Errorf("incorrect reply: Add: expected %d got %d", A+B, C)
					return
				}
				<-gate
				if atomic.AddInt32(&recv, -1) == 0 {
					close(res)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkEndToEnd(b *testing.B) {
	benchmarkEndToEnd(dialDirect, b)
}

func BenchmarkEndToEndHTTP(b *testing.B) {
	benchmarkEndToEnd(dialHTTP, b)
}

func BenchmarkEndToEndAsync(b *testing.B) {
	benchmarkEndToEndAsync(dialDirect, b)
}

func BenchmarkEndToEndAsyncHTTP(b *testing.B) {
	benchmarkEndToEndAsync(dialHTTP, b)
}
