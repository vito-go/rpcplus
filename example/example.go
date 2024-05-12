package main

import (
	"context"
	"fmt"
	"github.com/vito-go/rpcplus"

	"log"
	"net"
)

type Stu struct {
}
type RecvIntPtr int
type RecvInt int
type UserInfo struct {
	Name    string
	Age     int
	ClassId int
}

func (*RecvIntPtr) Hello(ctx context.Context, s string) error {
	log.Println("hello from RecvIntPtr")
	return nil
}
func (RecvInt) Hello(ctx context.Context, s string) error {
	log.Println("hello from RecvInt")
	return nil
}

func Add(ctx context.Context, x int, y int) (int64, error) {
	return int64(x + y), nil
}

func (s *Stu) GetUserInfoByUserId(ctx context.Context, userId int) (*UserInfo, error) {
	return &UserInfo{
		Name:    "Jack",
		Age:     31,
		ClassId: 1,
	}, nil
}
func (s *Stu) GetAgeByName(ctx context.Context, name string) (int, error) {
	return 31, nil
}

func (s *Stu) Post(ctx context.Context, id int) error {
	log.Println("do some thing here")
	return nil
}
func (s *Stu) UpdateUserInfo(ctx context.Context, u *UserInfo) (int64, error) {
	log.Println("do some thing here")
	return 100, nil
}
func main() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	exampleServer(listener)
	// wait for server starting
	_, err = net.Dial("tcp", listener.Addr().String())
	if err != nil {
		panic(err)
	}
	exampleClient(listener.Addr().String())
}

func exampleServer(listener net.Listener) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	s := rpcplus.NewServer()
	err := s.RegisterRecv(&Stu{})
	if err != nil {
		panic(err)
	}
	err = s.RegisterFunc("Add", Add)
	if err != nil {
		panic(err)
	}
	err = s.RegisterRecv(new(RecvIntPtr))
	if err != nil {
		panic(err)
	}
	err = s.RegisterRecv(RecvInt(1))
	if err != nil {
		panic(err)
	}
	err = s.RegisterFunc("Anonymous", func(ctx context.Context, x int, y int) (int64, error) {
		return int64(x * y), nil
	})
	if err != nil {
		panic(err)
	}

	log.Println("rpcplus: Starting server on port 8081")
	go s.Accept(listener)
}
func exampleClient(addr string) {
	dialer, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	cli := rpcplus.NewClient(dialer)
	//ctx := context.Background()
	var result UserInfo
	err = cli.Call("Stu.GetUserInfoByUserId", &result, 181)
	if err != nil {
		panic(err)
	}
	// OutPut UserInfo: {Name:Jack Age:31 ClassId:1}
	log.Println(fmt.Sprintf("UserInfo: %+v", result))
	var age int64
	err = cli.Call("Stu.GetAgeByName", &age, "Jack")
	if err != nil {
		panic(err)
	}
	// OutPut Age: 31
	log.Println(fmt.Sprintf("Age: %+v", age))
	err = cli.CallVoid("Stu.Post", 66)
	if err != nil {
		panic(err)
	}
	var addResult int
	err = cli.Call("Add", &addResult, 89, 64)
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 153
	log.Println(fmt.Sprintf("Add Result: %+v", addResult))
	err = cli.Call("Anonymous", &addResult, 89, 64)
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 5696
	log.Println(fmt.Sprintf("Anonymous Result: %+v", addResult))
	var userId int
	err = cli.Call("Stu.UpdateUserInfo", &userId, &UserInfo{
		Name:    "Jack",
		Age:     35,
		ClassId: 1,
	})
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 5696
	log.Println(fmt.Sprintf("Stu.UpdateUserInfo Result: %+v", userId))

	err = cli.CallVoid("RecvInt.Hello", "My World!")
	if err != nil {
		panic(err)
	}
	err = cli.CallVoid("RecvIntPtr.Hello", "My World!")
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 5696
}
