# 延迟数据管理

用于需要延迟得到的数据，例如异步请求的回调结果等。

## 安装

依赖 `go >= 1.18` ，初始化 go module 后直接安装

```bash
go get github.com/fioman/deferred-data
```

## 使用

```go
// 创建本地延迟数据管理
// 等待数据的协程和得到数据的协程需要在同一进程内

import "github.com/fioman/deferred-data"

d := deferred.NewLocalDeferred[int]()

// 创建基于redis的延迟数据管理
// 等待数据的协程和得到数据的协程可以跨进程，能确保应用无状态
// 适用于例如 serverless、水平扩展的场景

// 等待延迟的数据结果
value, err := d.Await("ticket")

// 得到数据
d.Resolve("ticket", 100)

// 得到的不是数据，是错误 
d.Reject("ticket", err)
```
