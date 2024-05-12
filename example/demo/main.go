package main

import (
	"context"
	"github.com/vito-go/rpcplus"
	"github.com/vito-go/rpcplus/example/demo/common"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	srv := rpcplus.NewServer()
	err = srv.RegisterRecv(new(Student))
	if err != nil {
		panic(err)
	}
	go srv.Accept(listener)
	// wait for server starting
	dialer, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		panic(err)
	}
	cli := rpcplus.NewClient(dialer)
	ctx := context.Background()
	var studentAPI common.StudentAPI = common.NewStudent(cli)
	s, err := studentAPI.GetNameById(ctx, 1)
	if err != nil {
		panic(err)
	}
	log.Println(s)

	userInfo, err := studentAPI.GetUserInfo(ctx, 1)
	if err != nil {
		panic(err)
	}
	log.Println(userInfo)
}
