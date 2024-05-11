package main

import (
	"context"
	"github/vito-go/rpcplus"
	"log"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"testing"
)
//go:generate  go test -bench=.

func BenchmarkRPCPlus(b *testing.B) {
	rpcPlusOnce.Do(startRPCPlusServer)
	dialer, err := net.Dial("tcp", rpcPlusServerAddr.Load().(string))
	if err != nil {
		panic(err)
	}
	cli := rpcplus.NewClient(dialer)
	for i := 0; i < b.N; i++ {
		var result int
		err := cli.Call("Add", &result, i, i)
		if err != nil {
			panic(err)
		}
		// result == i+i
	}
}
func BenchmarkRPC(b *testing.B) {
	rpcOnce.Do(startRPCServer)
	dialer, err := net.Dial("tcp", rpcServerAddr.Load().(string))
	if err != nil {
		panic(err)
	}
	cli := rpc.NewClient(dialer)
	for i := 0; i < b.N; i++ {
		var result int
		err := cli.Call("S.Add", i, &result)
		if err != nil {
			panic(err)
		}
		// result == i+i
	}
}

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

var (
	rpcPlusServerAddr atomic.Value // string
	rpcServerAddr     atomic.Value
	rpcPlusOnce       sync.Once
	rpcOnce           sync.Once
) // string

func startRPCPlusServer() {
	lis, err := net.Listen("tcp", "127.0.0.1:0") // any available address
	if err != nil {
		panic(err)
	}
	s := rpcplus.NewServer()
	err = s.RegisterFunc("Add", func(ctx context.Context, x int, y int) (int64, error) {
		return int64(x + y), nil
	})
	if err != nil {
		panic(err)
	}
	log.Println("rpcplus: Starting server on " + lis.Addr().String())
	go s.Accept(lis)
	rpcPlusServerAddr.Store(lis.Addr().String())
}

func startRPCServer() {
	lis, err := net.Listen("tcp", "127.0.0.1:0") // any available address
	if err != nil {
		panic(err)
	}
	s := rpc.NewServer()
	err = s.Register(new(S))
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
	log.Println("rpc: Starting server on " + lis.Addr().String())
	go s.Accept(lis)
	rpcServerAddr.Store(lis.Addr().String())
}

type S struct {
}

func (S) Add(a int, result *int) error {
	*result = a + a
	return nil
}
