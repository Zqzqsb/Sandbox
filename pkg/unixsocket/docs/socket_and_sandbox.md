# Unix Socket 与沙箱系统

## 概述

Unix Domain Socket 是沙箱系统中进程间通信的核心机制，它提供了安全、高效的本地进程间通信能力，特别适合在隔离环境中使用。

## 系统架构

```mermaid
flowchart TD
    subgraph 宿主环境
        A[宿主进程]
        B[Unix Socket]
    end
    
    subgraph 沙箱1
        C[沙箱进程1]
    end
    
    subgraph 沙箱2
        D[沙箱进程2]
    end
    
    A <-->|控制命令| B
    B <-->|文件描述符传递| C
    B <-->|凭证验证| D
```

## 通信机制

### 1. 基本通信流程

```mermaid
sequenceDiagram
    participant H as 宿主进程
    participant S as Unix Socket
    participant B as 沙箱进程
    
    H->>S: 创建 Socket 对
    H->>B: 启动沙箱进程
    H->>S: 发送控制命令
    S->>B: 传递命令
    B->>S: 发送响应
    S->>H: 返回结果
```

### 2. 文件描述符传递

```mermaid
flowchart LR
    subgraph 发送进程
        A[文件] --> B[Socket发送端]
    end
    
    subgraph 接收进程
        C[Socket接收端] --> D[新文件描述符]
    end
    
    B -->|SCM_RIGHTS| C
```

### 3. 凭证传递

```mermaid
flowchart TD
    A[进程凭证] --> B[Socket]
    B --> C{凭证验证}
    C -->|成功| D[允许通信]
    C -->|失败| E[拒绝访问]
```

## 安全机制

### 1. 访问控制

```mermaid
flowchart TD
    A[通信请求] --> B{检查凭证}
    B -->|有效| C[检查权限]
    B -->|无效| D[拒绝]
    C -->|允许| E[建立连接]
    C -->|禁止| D
```

### 2. 隔离机制

```mermaid
flowchart LR
    subgraph 网络隔离
        A[禁止网络访问]
    end
    
    subgraph IPC隔离
        B[Unix Socket]
        C[允许控制通信]
    end
    
    subgraph 文件系统隔离
        D[受限访问]
    end
```

## 沙箱集成

### 1. 资源控制

```mermaid
flowchart TD
    A[沙箱进程] --> B{资源限制}
    B --> C[CPU限制]
    B --> D[内存限制]
    B --> E[文件描述符限制]
    B --> F[Socket缓冲区限制]
```

### 2. 生命周期管理

```mermaid
stateDiagram-v2
    [*] --> 创建
    创建 --> 运行
    运行 --> 暂停
    暂停 --> 运行
    运行 --> 终止
    终止 --> [*]
    
    state 运行 {
        [*] --> 通信
        通信 --> 执行
        执行 --> 通信
    }
```

## 性能优化

### 1. 缓冲区管理

```mermaid
flowchart LR
    A[发送请求] --> B[检查缓冲区]
    B -->|足够| C[直接写入]
    B -->|不足| D[等待空间]
    D --> C
```

### 2. 消息批处理

```mermaid
flowchart TD
    A[消息队列] --> B[批处理器]
    B --> C{大小检查}
    C -->|达到阈值| D[批量发送]
    C -->|未达阈值| E[继续收集]
```

## 错误处理

### 1. 连接错误

```mermaid
flowchart TD
    A[连接异常] --> B{错误类型}
    B -->|断开| C[重连机制]
    B -->|超时| D[超时处理]
    B -->|权限| E[权限检查]
```

### 2. 数据错误

```mermaid
flowchart TD
    A[数据传输] --> B{数据验证}
    B -->|格式错误| C[请求重发]
    B -->|大小超限| D[分片处理]
    B -->|校验失败| E[错误恢复]
```

## 使用示例

### 1. 基本通信

```go
// 创建 Socket 对
sender, receiver, err := unixsocket.NewSocketPair()
if err != nil {
    log.Fatal(err)
}
defer sender.Close()
defer receiver.Close()

// 发送消息
msg := []byte("command")
if err := sender.SendMsg(msg, Msg{}); err != nil {
    log.Fatal(err)
}

// 接收消息
buf := make([]byte, 1024)
n, _, err := receiver.RecvMsg(buf)
if err != nil {
    log.Fatal(err)
}
```

### 2. 文件描述符传递

```go
// 发送文件描述符
file, err := os.Open("example.txt")
if err != nil {
    log.Fatal(err)
}
msg := Msg{
    Fds: []int{int(file.Fd())},
}
if err := sender.SendMsg([]byte("file"), msg); err != nil {
    log.Fatal(err)
}

// 接收文件描述符
buf := make([]byte, 1024)
n, msg, err := receiver.RecvMsg(buf)
if err != nil {
    log.Fatal(err)
}
newFile := os.NewFile(uintptr(msg.Fds[0]), "received")
```

## 最佳实践

### 1. 安全配置

- 严格的权限控制
- 最小权限原则
- 及时关闭未使用的连接

### 2. 性能调优

- 适当的缓冲区大小
- 批量处理消息
- 避免频繁的连接创建

### 3. 错误处理

- 完善的错误恢复机制
- 超时控制
- 资源清理

## 调试技巧

### 1. 日志记录

- 详细的错误信息
- 性能指标监控
- 通信状态追踪

### 2. 故障排除

- 连接状态检查
- 权限验证
- 资源使用监控
