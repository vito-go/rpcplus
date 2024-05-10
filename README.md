## [English](README.md) | [中文版](README-zh-hans.md)

# gorpc

gorpc is a robust enhancement of the standard Go RPC library.

## Key Features

- Direct registration of functions, including anonymous ones
- Ability to register all suitable methods associated with a receiver, typically a struct
  - For `suitable` definitions, please refer to  [Register Function](#Function)
- Support for functions with multiple input parameters
- Integrating `context.Context` within registered functions for advanced context management, e.g `context.WithTimeout`
  
These key enhancements contribute to a versatile and powerful RPC implementation.

## Function
RegisterFunc that satisfy the following conditions:
 - Have one or more arguments, with the first argument being of type context.Context
 - Return types: a single error or a pair with the second element an error
### Example
```go
  func Add(ctx context.Context, x int,y int) (int, error)

  func getAgeByName(ctx context.Context, name string) (int, error)

  func getAgeByClassIdAndName(ctx context.Context,classId int,name string) (int, error)

  type UserInfo struct{
      Name string
      Age int
      ClassId int
  }
	  
  func getUserInfoByUserId(ctx context.Context,userId int) (*UserInfo, error)

```

## Usage

- ### Server
    ```go
      package  main
      import (
          "context"
          "github.com/vito-go/gorpc"
          "log"
          "net"
      )
      // look at the example/example.go Stu and Add 
      func main()  {
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
    ```
  - ### Client
      ```go
      package main
      import (
        "context"
        "fmt"
        "github.com/vito-go/gorpc"
        "log"
        "net"
      )
      // look at the example/example.go UserInfo
      func main()  {
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
      }    
      ```
 