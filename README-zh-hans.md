## [English](README.md) | [中文版](README-zh-hans.md)

# gorpc
gorpc 是对标准 Go RPC 库的强健增强版, 进行了多项改进。

## 主要特点
- 直接注册函数，甚至是匿名函数
- 注册包含接收者的所有**适用**方法（通常是结构体）
  - 适用的定义请查阅[函数注册](#函数注册)
- 支持包含多个参数的函数
- 为注册的函数加入 `context.Context` 支持，用于更复杂的上下文管理, 例如`context.WithTimeout`
 
以上关键特性使得 gorpc 成为一个功能全面而强大的 RPC 框架。
## 函数注册
- 要求函数满足以下条件：
  - 一个参数或多个参数，首个为 context.Context 类型
  - 返回类型：单个 error 或两个值，第二个为 error 类型。

### 示例
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


## 使用
- ### 服务端
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
- ### 客户端
    ```go
        package main
        import (
            "context"
            "github.com/vito-go/gorpc"
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
        }
    
    
    ```
 