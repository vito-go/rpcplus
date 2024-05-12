package main

import (
	"context"
	"github.com/vito-go/rpcplus/example/demo/common"
)

type Student struct {
}

func (s *Student) GetUserInfo(ctx context.Context, userId int) (*common.UserInfo, error) {
	return &common.UserInfo{
		Name: "Jack",
		Id:   64,
		Age:  31,
	}, nil
}

var _ common.StudentAPI = (*Student)(nil)

func (s *Student) GetAgeByName(ctx context.Context, name string) (int, error) {
	return 1, nil
}

func (s *Student) GetNameById(ctx context.Context, id int) (string, error) {
	return "Jack", nil
}
