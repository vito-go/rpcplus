package rpcplus

// Copyright 2024 The github.com/vito-go. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package rpcplus provides access to a method or the exported methods of an object across a
network or other I/O connection.  A server registers an object, making it visible
as a service with the name of the type of the object.  After registration, the
methods will be accessible remotely.  A server may register multiple
objects (services) of different types, but it is an error to register multiple
objects of the same type.

Only methods that satisfy these criteria will be made available for remote access;
other methods will be ignored:

//   - Have one or more arguments, with the first argument being of type context.Context
//   - return value: if one, it is of type error; if two, the second is of type error

In effect, the method must look schematically like

	func (t *T) MethodName(ctx context.Context,argType T1) (*T2,error)
	or directly the method
	func MethodName(ctx context.Context,argType T1) (*T2,error)

where T1 and T2 can be marshaled by encoding/gob.
These requirements apply even if a different codec is used.
(In the future, these requirements may soften for custom codecs.)

The method's first argument is context.Context, the subsequent represents the arguments provided by the caller;
Given the return type, a single error or a pair with the second element an error, the first which represents the
result parameters to be returned to the caller.If an error is returned, the reply parameter
will not be sent back to the client.

The server may handle requests on a single connection by calling [ServeConn].  More
typically it will create a network listener and call [Accept] or, for an HTTP
listener, [HandleHTTP] and [http.Serve].

A client wishing to use the service establishes a connection and then invokes
[NewClient] on the connection. The convenience function [Dial] ([DialHTTP]) performs
both steps for a raw network connection (an HTTP connection).  The resulting
[Client] object has two methods, [Call] and Go, that specify the service and method to
call, a pointer containing the arguments, and a pointer to receive the result
parameters.

The Call method waits for the remote call to complete while the Go method
launches the call asynchronously and signals completion using the Call
structure's Done channel.

Unless an explicit codec is set up, package [encoding/gob] is used to
transport the data.

Look more examples at example/example.go

A server implementation will often provide a simple, type-safe wrapper for the
client.

The rpcplus package is a robust enhancement of the standard Go RPC library, which is frozen and is not accepting new features.
*/

import (
	"bufio"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"go/token"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"sync"
)

const (
	// Defaults used by HandleHTTP

	DefaultRPCPath   = "/_goRPCPlus_"
	DefaultDebugPath = "/debug/rpcplus"
)

type methodType struct {
	sync.Mutex        // protects counters
	name       string // name of service
	ArgTypes   []reflect.Type
	ReplyType  reflect.Type
	numCalls   uint
	function   reflect.Value
}

// Request is a header written before every RPC call. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Request struct {
	ServiceMethod string   // format: "Service.Method"
	Seq           uint64   // sequence number chosen by client
	next          *Request // for free list in Server
	ArgNum        int
}

// Response is a header written before every RPC return. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Response struct {
	ServiceMethod string    // echoes that of the Request
	Seq           uint64    // echoes that of the request
	Error         string    // error, if any.
	next          *Response // for free list in Server
}

// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map   // map[string]*service
	reqLock    sync.Mutex // protects freeReq
	freeReq    *Request
	respLock   sync.Mutex // protects freeResp
	freeResp   *Response

	// BaseContext optionally specifies a function that returns
	// the base context for incoming requests on this server.
	// The provided Listener is the specific Listener that's
	// about to start accepting requests.
	// If BaseContext is nil, the default is context.Background().
	// If non-nil, it must return a non-nil context.
	BaseContext func(net.Listener) context.Context

	// ConnContext optionally specifies a function that modifies
	// the context used for a new connection c. The provided ctx
	// is derived from the base context and has a ServerContextKey
	// value.
	ConnContext func(ctx context.Context, c net.Conn) context.Context

	// ConnContext optionally specifies a function that modifies
	// the context used for a new connection c. The provided ctx
	// is derived from the base context and has a ServerContextKey
	// value.
	RequestContext func(ctx context.Context) context.Context
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer, so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "rpcplus context value " + k.name }

var (
	// ServerContextKey is a context key. It can be used when using Accept to start a server  in RPC function
	// with Context.Value to access the server that
	// started the handler. The associated value will be of
	// type *Server.
	ServerContextKey = &contextKey{"rpcplus-server"}

	// LocalAddrContextKey is a context key. It can be used when using Accept to start a server in RPC function
	// with Context.Value to access the local address the connection arrived at.
	// The associated value will be of type net.Addr.
	LocalAddrContextKey = &contextKey{"local-addr"}
)

// NewServer returns a new [Server].
func NewServer() *Server {
	return &Server{}
}

// DefaultServer is the default instance of [*Server].
var DefaultServer = NewServer()

// RegisterFunc that satisfy the following conditions:
//   - Have one or more arguments, with the first argument being of type context.Context
//   - return value: if one, it is of type error; if two, the second is of type error
//
// e.g.
//
//	func Add(ctx context.Context, x int,y int) (int, error)
//	func GetAgeByName(ctx context.Context, name string) (int, error)
//	func GetAgeByClassIdAndName(ctx context.Context,classId int,name string) (int, error)
//	type UserInfo struct{
//		Name string
//		Age int
//		ClassId int
//	}
//	func getUserInfoByUserId(ctx context.Context,userId string) (*UserInfo, error)
func (server *Server) RegisterFunc(serviceName string, function any) error {
	f := reflect.ValueOf(function)
	err := server.checkMethod(serviceName, f)
	if err != nil {
		return err
	}
	var ArgTypes []reflect.Type
	for i := 0; i < f.Type().NumIn(); i++ {
		ArgTypes = append(ArgTypes, f.Type().In(i))
	}
	out := f.Type().NumOut()
	var replyType reflect.Type
	if out == 2 {
		replyType = f.Type().Out(0)
	}
	m := &methodType{
		name:      serviceName,
		ArgTypes:  ArgTypes,
		ReplyType: replyType,
		function:  f,
	}
	if _, dup := server.serviceMap.LoadOrStore(serviceName, m); dup {
		return fmt.Errorf("service already defined: " + serviceName)
	}
	if debugLog {
		log.Printf("rpcplus: registered service %q, type: %s, method: %s\n", serviceName, f.Type(), m.name)
	}
	return nil
}
func (server *Server) checkMethod(serviceName string, f reflect.Value) error {
	if f.Kind() != reflect.Func {
		return errors.New("kind is not a function: " + f.Kind().String())
	}
	in := f.Type().NumIn()
	if in < 1 {
		return fmt.Errorf("function %q must have at least one input parameter, but got %d", serviceName, in)
	}
	ok := f.Type().In(0) == reflect.TypeOf((*context.Context)(nil)).Elem()
	if !ok {
		return fmt.Errorf("function %q first param's kind must be context", serviceName)
	}
	out := f.Type().NumOut()

	switch out {
	case 0:
		return fmt.Errorf("function %q must have at least one return value, but got 0", serviceName)
	case 1:
		ok = f.Type().Out(0) == reflect.TypeOf((*error)(nil)).Elem()
		if !ok {
			return fmt.Errorf("function %q param's kind must be error", serviceName)
		}
	case 2:
		ok = f.Type().Out(1) == reflect.TypeOf((*error)(nil)).Elem()
		if !ok {
			return fmt.Errorf("function %q second param's kind must be error", serviceName)
		}
	default:
		return fmt.Errorf("function %q should have 1 or 2 output parameter, but got %d", serviceName, out)
	}
	return nil
}

// Deprecated: please use RegisterRecvWithName
func (server *Server) RegisterName(recvName string, receiver any) error {
	return server.RegisterRecvWithName(recvName, receiver)
}
func (server *Server) RegisterRecvWithName(recvName string, receiver any) error {
	return server.registerRecvWithName(recvName, receiver)
}

// Deprecated: please use RegisterRecv
func (server *Server) Register(receiver any) error {
	return server.RegisterRecv(receiver)
}
func (server *Server) RegisterRecv(receiver any) error {
	v := reflect.TypeOf(receiver)

	var recvName string
	switch v.Kind() {
	case reflect.Ptr:
		recvName = v.Elem().Name()
	default:
		recvName = v.Name()
	}
	return server.registerRecvWithName(recvName, receiver)
}
func (server *Server) registerRecvWithName(recvName string, receiver any) error {
	t := reflect.TypeOf(receiver)
	//if t.Kind() != reflect.Ptr {
	//	return errors.New("receiver must be ptr")
	//}
	if t.NumMethod() == 0 {
		return errors.New("receiver must have at least one method")
	}
	v := reflect.ValueOf(receiver)
	var atLeastOne bool
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		serviceName := recvName + "." + m.Name
		err := server.RegisterFunc(serviceName, v.Method(i).Interface())
		//err := server.RegisterFunc(serviceName, v.Method(i))
		if err != nil {
			log.Println("rpcplus: [WARN] registered service ignored:", err.Error())
			continue
		}
		atLeastOne = true
	}
	if !atLeastOne {
		return errors.New("receiver must have at least one suitable method")
	}
	return nil
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}

// logRegisterError specifies whether to log problems during method registration.
// To debug registration, recompile the package with this set to true.
const logRegisterError = false

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.
var invalidRequest = struct{}{}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply any, codec ServerCodec, errmsg string) {
	resp := server.getResponse()
	// Encode the response header
	resp.ServiceMethod = req.ServiceMethod
	if errmsg != "" {
		resp.Error = errmsg
		reply = invalidRequest
	}
	resp.Seq = req.Seq
	sending.Lock()
	err := codec.WriteResponse(resp, reply)
	if debugLog && err != nil {
		log.Println("rpcplus: writing response:", err)
	}
	sending.Unlock()
	server.freeResponse(resp)
}

func (m *methodType) NumCalls() (n uint) {
	m.Lock()
	n = m.numCalls
	m.Unlock()
	return n
}

var emptyReply = struct{}{}

func (mtype *methodType) call(ctx context.Context, server *Server, sending *sync.Mutex, wg *sync.WaitGroup, req *Request, codec ServerCodec, argv ...reflect.Value) {
	if wg != nil {
		defer wg.Done()
	}
	mtype.Lock()
	mtype.numCalls++
	mtype.Unlock()
	function := mtype.function
	// Invoke the method, providing a new value for the reply.
	callArgs := make([]reflect.Value, 0, len(argv)+1)
	callArgs = append(callArgs, reflect.ValueOf(ctx))

	for i := 0; i < len(argv); i++ {
		if mtype.ArgTypes[i+1].Kind() == reflect.Pointer {
			callArgs = append(callArgs, argv[i])
		} else {
			if argv[i].Kind() == reflect.Struct {
				callArgs = append(callArgs, argv[i])
				continue
			}
			callArgs = append(callArgs, argv[i].Elem())
		}
	}

	returnValues := function.Call(callArgs)

	var reply any = emptyReply
	var errInter any
	errmsg := ""
	switch len(returnValues) {
	case 1:
		errInter = returnValues[0].Interface()
		if errInter != nil {
			errmsg = errInter.(error).Error()
		}
	case 2:
		reply = returnValues[0].Interface()
		errInter = returnValues[1].Interface()
		if errInter != nil {
			errmsg = errInter.(error).Error()
		}
	}
	// The return value for the method is an error.

	server.sendResponse(sending, req, reply, codec, errmsg)
	server.freeRequest(req)
}

type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

func (c *gobServerCodec) ReadRequestHeader(r *Request) error {
	return c.dec.Decode(r)
}

func (c *gobServerCodec) ReadRequestBody(args ...any) error {
	var stopErr error
	for i := 0; i < len(args); i++ {
		arg := args[i]
		err := c.dec.Decode(arg)
		if err != nil {
			stopErr = err
			continue
		}
	}
	if stopErr != nil {
		return stopErr
	}
	return nil
}

func (c *gobServerCodec) WriteResponse(r *Response, body any) (err error) {
	if err = c.enc.Encode(r); err != nil {
		if c.encBuf.Flush() == nil {
			// Gob couldn't encode the header. Should not happen, so if it does,
			// shut down the connection to signal that the connection is broken.
			log.Println("rpcplus: gob error encoding response:", err)
			c.Close()
		}
		return
	}
	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			// Was a gob problem encoding the body but the header has been written.
			// Shut down the connection to signal that the connection is broken.
			log.Println("rpcplus: gob error encoding body:", err)
			c.Close()
		}
		return
	}
	return c.encBuf.Flush()
}

func (c *gobServerCodec) Close() error {
	if c.closed {
		// Only call c.rwc.Close once; otherwise the semantics are undefined.
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

// ServeConn runs the server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection. To use an alternate codec, use [ServeCodec].
// See [NewClient]'s comment for information about concurrent access.
func (server *Server) ServeConn(ctx context.Context, conn io.ReadWriteCloser) {
	buf := bufio.NewWriter(conn)
	srv := &gobServerCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}
	server.ServeCodec(ctx, srv)
}

// ServeCodec is like [ServeConn] but uses the specified codec to
// decode requests and encode responses.
func (server *Server) ServeCodec(ctx context.Context, codec ServerCodec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		reqCtx := ctx
		if server.RequestContext != nil {
			reqCtx = server.RequestContext(ctx)
		}
		mtype, req, argv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if debugLog && err != io.EOF {
				log.Println("rpcplus:", err)
			}
			if !keepReading {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err.Error())
				server.freeRequest(req)
			}
			continue
		}
		wg.Add(1)
		go mtype.call(reqCtx, server, sending, wg, req, codec, argv...)
	}
	// We've seen that there are no more requests.
	// Wait for responses to be sent before closing codec.
	wg.Wait()
	codec.Close()
}

// ServeRequest is like [ServeCodec] but synchronously serves a single request.
// It does not close the codec upon completion.
func (server *Server) ServeRequest(codec ServerCodec) error {
	sending := new(sync.Mutex)
	mtype, req, argv, keepReading, err := server.readRequest(codec)
	if err != nil {
		if !keepReading {
			return err
		}
		// send a response if we actually managed to read a header.
		if req != nil {
			server.sendResponse(sending, req, invalidRequest, codec, err.Error())
			server.freeRequest(req)
		}
		return err
	}
	reqCtx := context.Background()
	if server.RequestContext != nil {
		reqCtx = server.RequestContext(reqCtx)
	}
	mtype.call(reqCtx, server, sending, nil, req, codec, argv...)
	return nil
}

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) readRequest(codec ServerCodec) (mtype *methodType, req *Request, argv []reflect.Value, keepReading bool, err error) {
	mtype, req, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		codec.ReadRequestBody(make([]any, req.ArgNum)...)
		return
	}
	argv = make([]reflect.Value, 0, req.ArgNum)
	bodyValues := make([]any, 0, req.ArgNum)
	argIsValues := make([]bool, 0, req.ArgNum)
	for i := 1; i < len(mtype.ArgTypes); i++ {
		var a reflect.Value
		if mtype.ArgTypes[i].Kind() == reflect.Pointer {
			a = reflect.New(mtype.ArgTypes[i].Elem())
			argIsValues = append(argIsValues, false)
		} else {
			a = reflect.New(mtype.ArgTypes[i])
			argIsValues = append(argIsValues, true)
		}

		argv = append(argv, a)
		bodyValues = append(bodyValues, a.Interface())
	}

	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(bodyValues...); err != nil {
		return
	}

	for i := 0; i < len(argIsValues); i++ {
		if argIsValues[i] {
			if argv[i].Kind() == reflect.Pointer && argv[i].Elem().Kind() == reflect.Struct {
				argv[i] = argv[i].Elem()
			}
		}
	}

	return
}

func (server *Server) readRequestHeader(codec ServerCodec) (mtype *methodType, req *Request, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			return
		}
		err = errors.New("rpcplus: server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully. If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	serviceName := req.ServiceMethod
	// Look up the request.
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpcplus: can't find method")
		return
	}
	mtype = svci.(*methodType)
	if len(mtype.ArgTypes) != req.ArgNum+1 {
		err = fmt.Errorf("rpcplus: wrong number inputs")
	}

	return
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection. Accept blocks until the listener
// returns a non-nil error. The caller typically invokes Accept in a
// go statement.
func (server *Server) Accept(lis net.Listener) {
	baseCtx := context.Background()
	if server.BaseContext != nil {
		baseCtx = server.BaseContext(lis)
		if baseCtx == nil {
			panic("BaseContext returned a nil context")
		}
	}
	ctx := context.WithValue(baseCtx, ServerContextKey, server)
	for {
		conn, err := lis.Accept()
		if err != nil {
			// TODO here should continue or return ?
			log.Print("rpcplus.Serve: accept:", err.Error())
			return
		}
		connCtx := context.WithValue(ctx, LocalAddrContextKey, conn.LocalAddr())
		if cc := server.ConnContext; cc != nil {
			connCtx = cc(connCtx, conn)
			if connCtx == nil {
				panic("ConnContext returned nil")
			}
		}
		go server.ServeConn(connCtx, conn)
	}
}

// Register publishes the receiver's methods in the [DefaultServer].
// Deprecated: please use RegisterRecv
func Register(rcvr any) error     { return DefaultServer.RegisterRecv(rcvr) }
func RegisterRecv(rcvr any) error { return DefaultServer.RegisterRecv(rcvr) }

func RegisterFunc(serviceName string, f any) error { return DefaultServer.RegisterFunc(serviceName, f) }

// RegisterName is like [Register] but uses the provided name for the type
// instead of the receiver's concrete type.
func RegisterName(name string, rcvr any) error {
	return DefaultServer.registerRecvWithName(name, rcvr)
}

// A ServerCodec implements reading of RPC requests and writing of
// RPC responses for the server side of an RPC session.
// The server calls [ServerCodec.ReadRequestHeader] and [ServerCodec.ReadRequestBody] in pairs
// to read requests from the connection, and it calls [ServerCodec.WriteResponse] to
// write a response back. The server calls [ServerCodec.Close] when finished with the
// connection. ReadRequestBody may be called with a nil
// argument to force the body of the request to be read and discarded.
// See [NewClient]'s comment for information about concurrent access.
type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(...any) error
	WriteResponse(*Response, any) error

	// Close can be called multiple times and must be idempotent.
	Close() error
}

// ServeConn runs the [DefaultServer] on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection. To use an alternate codec, use [ServeCodec].
// See [NewClient]'s comment for information about concurrent access.
func ServeConn(conn io.ReadWriteCloser) {
	DefaultServer.ServeConn(context.Background(), conn)
}

// ServeCodec is like [ServeConn] but uses the specified codec to
// decode requests and encode responses.
func ServeCodec(codec ServerCodec) {
	DefaultServer.ServeCodec(context.Background(), codec)
}

// ServeRequest is like [ServeCodec] but synchronously serves a single request.
// It does not close the codec upon completion.
func ServeRequest(codec ServerCodec) error {
	return DefaultServer.ServeRequest(codec)
}

// Accept accepts connections on the listener and serves requests
// to [DefaultServer] for each incoming connection.
// Accept blocks; the caller typically invokes it in a go statement.
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

// Can connect to RPC service using HTTP CONNECT to rpcPath.
var connected = "200 Connected to Go RPC"

// ServeHTTP implements an [http.Handler] that answers RPC requests.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpcplus hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	server.ServeConn(req.Context(), conn)
}

// HandleHTTP registers an HTTP handler for RPC messages on rpcPath,
// and a debugging handler on debugPath.
// It is still necessary to invoke [http.Serve](), typically in a go statement.
func (server *Server) HandleHTTP(rpcPath, debugPath string) {
	http.Handle(rpcPath, server)
	http.Handle(debugPath, debugHTTP{server})
}

// HandleHTTP registers an HTTP handler for RPC messages to [DefaultServer]
// on [DefaultRPCPath] and a debugging handler on [DefaultDebugPath].
// It is still necessary to invoke [http.Serve](), typically in a go statement.
func HandleHTTP() {
	DefaultServer.HandleHTTP(DefaultRPCPath, DefaultDebugPath)
}
