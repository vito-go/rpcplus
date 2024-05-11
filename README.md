## [English](README.md) | [中文版](README-zh-hans.md)

# rpcplus

rpcplus is a refined library that builds upon the Go language's standard rpc package,
offering a suite of enhancements for an improved RPC experience.
## Key Features

- Support for direct function registration on the RPC server for simplified service provisioning.
- Ability to register anonymous functions, offering flexibility in programming.
- Ability to register all suitable methods associated with a receiver, typically a struct
  - For `suitable` definitions, please refer to  [Register Function](#Function)
- Support for functions with multiple input parameters
- Support for passing a context.Context parameter, enabling handling of timeouts, cancellations, and other context-related operations.
- Support for Go-style return value patterns, allowing functions to return  `(T, error)` or just `error`.
- Reserves jsonrpc support, facilitating cross-language communication

These key enhancements contribute to a versatile and powerful RPC implementation.

## Usage

- ### Server
    ```go
      package  main
      import (
          "context"
          "github.com/vito-go/rpcplus"
          "log"
          "net"
      )
      // look at the example/example.go Stu and Add 
      func main()  {
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
          log.Println("rpcplus: Starting server on port 8081")
          s.Serve(lis)
      }
    ```
- ### Client
    ```go
    package main
    import (
      "context"
      "fmt"
      "github.com/vito-go/rpcplus"
      "log"
      "net"
    )
    // look at the example/example.go UserInfo
    func main()  {
        dialer, err := net.Dial("tcp", "127.0.0.1:8081")
        if err != nil {
            panic(err)
        }
        cli := rpcplus.NewClient(dialer)
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

## Function
RegisterFunc that satisfy the following conditions:
- Have one or more arguments, with the first argument being of type context.Context
- Return types: a single error or a pair with the second element an error

- ### Function Example

```go
  func add(ctx context.Context, x int,y int) (int, error)

  func GetAgeByName(ctx context.Context, name string) (int, error)

  func GetAgeByClassIdAndName(ctx context.Context,classId int,name string) (int, error)

  type UserInfo struct{
      Name string
      Age int
      ClassId int
  }
	  
  func GetUserInfoByUserId(ctx context.Context,userId int) (*UserInfo, error)

  func UpdateUserInfo(ctx context.Context, u *UserInfo) (int64, error)

```

- ### Service Function List (for test)
- #### from debugHTTP
  <table border="1" cellpadding="5">
  <tbody><tr><th align="center">Service</th><th align="center">MethodType</th><th align="center">Calls</th>
          </tr><tr>
          <td align="left" font="fixed">Arith.Add</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.Div</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.Error</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.Mul</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.Scan</td>
          <td align="left" font="fixed">func(context.Context, string) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.SleepMilli</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Arith.String</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*string, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">Embed.Exported</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.Add</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.Div</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.Error</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.Mul</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.Scan</td>
          <td align="left" font="fixed">func(context.Context, string) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.SleepMilli</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">net.rpcplus.Arith.String</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*string, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.Add</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.Div</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.Error</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.Mul</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.Scan</td>
          <td align="left" font="fixed">func(context.Context, string) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.SleepMilli</td>
          <td align="left" font="fixed">func(context.Context, rpcplus.Args) (*rpcplus.Reply, error)</td>
          <td align="center">0</td></tr>
          <tr>
          <td align="left" font="fixed">newServer.Arith.String</td>
          <td align="left" font="fixed">func(context.Context, *rpcplus.Args) (*string, error)</td>
          <td align="center">0</td></tr>
  </tbody></table>

## Testing

All test cases have been executed and passed successfully, ensuring the quality and reliability of the codebase. Contributors are encouraged to run tests before making submissions to maintain project stability.
