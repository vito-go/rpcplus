## [English](README.md) | [中文版](README-zh-hans.md)

# rpcplus
rpcplus 是对标准 Go RPC 库的增强版, 进行了多项改进。

## 主要特点
- 直接注册函数，甚至是匿名函数
- 注册包含接收者的所有**适用**方法（通常是结构体）
  - 适用的定义请查阅[函数注册](#函数注册)
- 支持包含多个参数的函数
- 为注册的函数加入 `context.Context` 支持，用于更复杂的上下文管理, 例如`context.WithTimeout`
 
以上关键特性使得 rpcplus 成为一个功能全面而强大的 RPC 框架。
## 函数注册
- 要求函数满足以下条件：
  - 一个参数或多个参数，首个为 context.Context 类型
  - 返回类型：单个 error 或两个值，第二个为 error 类型。

### 示例
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


## 使用
- ### 服务端
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
- ### 客户端
    ```go
        package main
        import (
            "context"
            "github.com/vito-go/rpcplus"
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
        }
    
    
    ```


## 服务方法列表
- ### 来自 debugHTTP
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
