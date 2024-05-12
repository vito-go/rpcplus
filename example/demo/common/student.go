// Package common should be a public repository that the server side provides to the client.
package common

import (
	"context"
	"github.com/vito-go/rpcplus"
)

type Student struct {
	cli *rpcplus.Client
}

var _ StudentAPI = (*Student)(nil)

type StudentAPI interface {
	GetAgeByName(ctx context.Context, name string) (int, error)
	GetNameById(ctx context.Context, id int) (string, error)
	GetUserInfo(ctx context.Context, userId int) (*UserInfo, error)
}

func NewStudent(cli *rpcplus.Client) *Student {
	return &Student{cli: cli}
}

type UserInfo struct {
	Name string
	Id   int
	Age  int
}

func (c *Student) GetAgeByName(ctx context.Context, name string) (int, error) {
	var result = new(int)
	err := c.cli.Call("Student.GetAgeByName", result, name)
	return *result, err
}

func (c *Student) GetNameById(ctx context.Context, id int) (string, error) {
	var result = new(string)
	err := c.cli.Call("Student.GetNameById", result, id)
	return *result, err
}
func (c *Student) GetUserInfo(ctx context.Context, userId int) (*UserInfo, error) {
	var result = new(UserInfo)
	err := c.cli.Call("Student.GetUserInfo", result, userId)
	return result, err
}
