// Copyright 2024 github.com/vito-go. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gorpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
)

type R struct {
	msg []byte // Not exported, so R does not work with gob.
}

type S struct{}

func (s *S) Recv(ctx context.Context, nul struct{}) (*R, error) {

	return &R{[]byte("foo")}, nil
}

func TestGobError(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("no error")
		}
		if !strings.Contains(err.(error).Error(), "reading body EOF") {
			t.Fatal("expected `reading body EOF', got", err)
		}
	}()
	server := NewServer()
	err := server.RegisterRecv(new(S))
	if err != nil {
		panic(err)
	}

	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go server.Serve(listen)
	dialer, err := net.Dial("tcp", listen.Addr().String())
	if err != nil {
		panic(err)
	}
	client := NewClient(dialer)
	ctx := context.Background()
	var reply R
	err = client.Call(ctx, "S.Recv", &reply, &struct{}{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", reply)
	client.Close()

	listen.Close()
}
