package main

import (
	"context"
	"fmt"
	"github.com/vito-go/gorpc"
	"log"
	"net"
	"time"
)

type Stu struct {
}
type UserInfo struct {
	Name    string
	Age     int
	ClassId int
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
func (s *Stu) UpdateUserInfo(ctx context.Context, u *UserInfo) (int64, error) {
	log.Println("do some thing here")
	return 100, nil
}
func (s *Stu) Post(ctx context.Context, id int) error {
	log.Println("do some thing here")
	return nil
}
func main() {
	go exampleServer()
	time.Sleep(1 * time.Second)
	exampleClient()
}

func exampleServer() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	s := gorpc.NewServer()
	err := s.RegisterRecv(&Stu{})
	if err != nil {
		panic(err)
	}
	err = s.RegisterFunc("Add", Add)
	if err != nil {
		panic(err)
	}
	err = s.RegisterFunc("Anonymous", func(ctx context.Context, x int, y int) (int64, error) {
		return int64(x * y), nil
	})
	if err != nil {
		panic(err)
	}
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	log.Println("gorpc: Starting server on port 8081")
	s.Serve(lis)
}
func exampleClient() {
	dialer, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		panic(err)
	}
	cli := gorpc.NewClient(dialer)
	ctx := context.Background()
	var result UserInfo
	err = cli.Call(ctx, "Stu.GetUserInfoByUserId", &result, 181)
	if err != nil {
		panic(err)
	}
	// OutPut UserInfo: {Name:Jack Age:31 ClassId:1}
	log.Println(fmt.Sprintf("UserInfo: %+v", result))
	var age int64
	err = cli.Call(ctx, "Stu.GetAgeByName", &age, "Jack")
	if err != nil {
		panic(err)
	}
	// OutPut Age: 31
	log.Println(fmt.Sprintf("Age: %+v", age))
	err = cli.CallVoid(ctx, "Stu.Post", 66)
	if err != nil {
		panic(err)
	}
	var addResult int
	err = cli.Call(ctx, "Add", &addResult, 89, 64)
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 153
	log.Println(fmt.Sprintf("Add Result: %+v", addResult))
	err = cli.Call(ctx, "Anonymous", &addResult, 89, 64)
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 5696
	log.Println(fmt.Sprintf("Anonymous Result: %+v", addResult))
	var userId int
	err = cli.Call(ctx, "Stu.UpdateUserInfo", &userId, &UserInfo{
		Name:    "Jack",
		Age:     35,
		ClassId: 1,
	})
	if err != nil {
		panic(err)
	}
	// OutPut Add Result: 5696
	log.Println(fmt.Sprintf("Stu.UpdateUserInfo Result: %+v", userId))

}
