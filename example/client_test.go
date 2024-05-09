package main

import (
	"context"
	"github.com/vito-go/gorpc"
	"net"
	"testing"
)

func BenchmarkName(b *testing.B) {
	dialer, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		panic(err)
	}
	cli := gorpc.NewClient(dialer)
	ctx := context.Background()
	var result int64
	for i := 0; i < b.N; i++ {
		err = cli.Call(ctx, "Stu.Nothing", &result, i)
		if err != nil {
			panic(err)
		}
	}
}
