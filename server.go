package gorpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

type Server struct {
	serviceMap sync.Map // map[string]RPCFunc
}

func NewServer() *Server {
	return &Server{serviceMap: sync.Map{}}
}

func (server *Server) Accept(listener net.Listener) {
	server.Serve(listener)
}
func (server *Server) Serve(listener net.Listener) {
	var wg sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			break
		}
		codec := newGobServerCodec(conn)
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.serveConn(codec)
		}()
	}
	wg.Wait()
}

type Request struct {
	NoReply       bool
	ServiceMethod string
	ArgsNum       int
	Seq           uint64
}

type Response struct {
	Seq     uint64
	Code    int    // 200 for success, 404 for service data not found
	Message string //  error, if any.
}

type methodType struct {
	name      string // name of service
	argTypes  []reflect.Type
	replyType reflect.Type  // can be nil if only one return value
	function  reflect.Value // function
}

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
		argTypes:  ArgTypes,
		replyType: replyType,
		function:  f,
	}
	if _, dup := server.serviceMap.LoadOrStore(serviceName, m); dup {
		return fmt.Errorf("service already defined: " + serviceName)
	}
	log.Printf("gorpc: registered service %q, type: %s, method: %s\n", serviceName, f.Type(), m.name)
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

func (server *Server) RegisterRecvWithName(recvName string, receiver any) error {
	return server.registerRecvWithName(recvName, receiver)
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
			log.Println("gorpc: [WARN] registered service ignored:", err.Error())
			continue
		}
		atLeastOne = true
	}
	if !atLeastOne {
		return errors.New("receiver must have at least one suitable method")
	}
	return nil
}

func (mtype *methodType) call(ctx context.Context, args ...reflect.Value) (result any, err error) {
	if len(mtype.argTypes) != len(args)+1 {
		return nil, errors.New("gorpc: invalid number of arguments")
	}
	f := mtype.function
	callArgs := make([]reflect.Value, 0, len(mtype.argTypes))
	callArgs = append(callArgs, reflect.ValueOf(ctx))

	for i := 0; i < len(args); i++ {
		if mtype.argTypes[i+1].Kind() == reflect.Pointer {
			callArgs = append(callArgs, args[i])
		} else {
			callArgs = append(callArgs, args[i].Elem())
		}
	}

	values := f.Call(callArgs)
	if len(values) != 2 {
		return nil, fmt.Errorf("gorpc: invalid return number. method %s length error: %v", mtype.name, len(values))
	}
	result = values[0].Interface()
	vErr := values[1].Interface()
	if vErr == nil {
		return values[0].Interface(), nil
	}
	var ok bool
	// err is no nil
	err, ok = values[1].Interface().(error)
	if !ok {
		return nil, fmt.Errorf("gorpc: invalid result, not a error: %+v", values[1].Interface())
	}
	return result, err
}

func (mtype *methodType) callVoid(ctx context.Context, args ...reflect.Value) (err error) {
	if len(mtype.argTypes) != len(args)+1 {
		return errors.New("gorpc: invalid number of arguments")
	}
	f := mtype.function
	callArgs := make([]reflect.Value, 0, len(args))
	callArgs = append(callArgs, reflect.ValueOf(ctx))
	for i := 0; i < len(args); i++ {
		if mtype.argTypes[i+1].Kind() == reflect.Pointer {
			callArgs = append(callArgs, args[i])
		} else {
			callArgs = append(callArgs, args[i].Elem())
		}
	}
	values := f.Call(callArgs)
	if len(values) != 1 {
		return fmt.Errorf("gorpc: invalid return number. method %s length error: %v", mtype.name, len(values))
	}
	vErr := values[0].Interface()
	if vErr == nil {
		return nil
	}
	var ok bool
	// err is no nil
	err, ok = values[0].Interface().(error)
	if !ok {
		return fmt.Errorf("gorpc: invalid result, not a error: %+v", values[1].Interface())
	}
	return err
}

func (server *Server) writeResponse(codec CodecServer, mutex *sync.Mutex, response *Response, body any) error {
	mutex.Lock()
	defer mutex.Unlock()
	err := codec.WriteResponse(response, body)
	if err != nil {
		codec.Close()
		return err
	}
	return nil
}

var invalidRequest = struct{}{}

func (server *Server) serveConn(codec CodecServer) {
	defer codec.Close()
	wg := new(sync.WaitGroup)
	mux := &sync.Mutex{}
	for {
		ctx := context.Background()
		request, err := codec.ReadRequestHeader()
		if err != nil {
			if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
				return
			}
			err = errors.New("gorpc: server cannot decode request: " + err.Error())
			log.Printf("read request header: %v", err)
			break
		}
		sf, ok := server.serviceMap.Load(request.ServiceMethod)
		if !ok {
			_ = codec.ReadRequestBody(make([]any, request.ArgsNum)...)
			// todo arg>0
			err = server.writeResponse(codec, mux, &Response{Code: 404, Seq: request.Seq, Message: "rpcplus: can't find method: " + request.ServiceMethod}, invalidRequest)
			if err != nil {
				return
			}
			continue
		}
		mtype := sf.(*methodType)
		// first is context
		if request.ArgsNum+1 < len(mtype.argTypes) {
			_ = codec.ReadRequestBody(make([]any, request.ArgsNum)...)
			if err = server.writeResponse(codec, mux, &Response{Code: 403, Seq: request.Seq, Message: "gorpc: call with too few input arguments"}, invalidRequest); err != nil {
				return
			}
			continue
		} else if request.ArgsNum+1 > len(mtype.argTypes) {
			_ = codec.ReadRequestBody(make([]any, request.ArgsNum)...)
			if err = server.writeResponse(codec, mux, &Response{Code: 403, Seq: request.Seq, Message: "gorpc: call with too many input arguments"}, invalidRequest); err != nil {
				return
			}
			continue
		}
		args := make([]reflect.Value, 0, request.ArgsNum)
		bodyArgs := make([]any, 0, request.ArgsNum)
		argVTrues := make([]bool, 0, request.ArgsNum)
		for i := 1; i < len(mtype.argTypes); i++ {
			argType := mtype.argTypes[i]
			var argv reflect.Value
			argIsValue := false // if true, need to indirect before calling.
			if argType.Kind() == reflect.Pointer {
				argv = reflect.New(argType.Elem())
			} else {
				argv = reflect.New(argType)
				argIsValue = true
			}
			args = append(args, argv)
			bodyArgs = append(bodyArgs, argv.Interface())
			argVTrues = append(argVTrues, argIsValue)
		}
		err = codec.ReadRequestBody(bodyArgs...)
		if err != nil {
			err = server.writeResponse(codec, mux, &Response{Code: 403, Seq: request.Seq, Message: err.Error()}, invalidRequest)
			if err != nil {
				return
			}
			continue
		}
		for i := 0; i < len(args); i++ {
			if argVTrues[i] {
				args[i] = args[i]
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			var reply any = invalidRequest
			if request.NoReply {
				err = mtype.callVoid(ctx, args...)
			} else {
				reply, err = mtype.call(ctx, args...)
			}
			if err != nil {
				if err = server.writeResponse(codec, mux, &Response{Seq: request.Seq, Code: 403, Message: err.Error()}, invalidRequest); err != nil {
					_ = codec.Close()
					return
				}
				return
			}
			if err = server.writeResponse(codec, mux, &Response{Seq: request.Seq, Message: "", Code: 200}, reply); err != nil {
				_ = codec.Close()
				return
			}
		}()
	}
	// We've seen that there are no more requests.
	// Wait for responses to be sent before closing codec.
	wg.Wait()
}
