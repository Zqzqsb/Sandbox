# Container 模块架构设计

## 1. 核心架构

```mermaid
graph TB
    subgraph "Host Process"
        A[Container Client] --> B[Unix Socket]
    end
    
    subgraph "Container Process"
        C[Container Server] --> D[Init Process]
        D --> E[Command Handler]
    end
    
    B <--> |Protocol| C
    
    style A fill:#f9f,stroke:#333,stroke-width:2px
    style C fill:#bbf,stroke:#333,stroke-width:2px
```

## 2. 通信流程

```mermaid
sequenceDiagram
    participant Client as Container Client
    participant Server as Container Server
    
    Client->>Server: Command (ping/conf/open/delete/reset/execve)
    Note right of Server: Command Processing
    Server-->>Client: Reply (success/error/finished)
    
    rect rgb(200, 150, 255)
    Note over Client,Server: File Descriptors & PIDs
    end
```

## 3. 命令处理

```mermaid
graph LR
    subgraph "Commands"
        A[Command] --> B[Ping]
        A --> C[Conf]
        A --> D[Open]
        A --> E[Delete]
        A --> F[Reset]
        A --> G[Execve]
    end
    
    subgraph "Replies"
        H[Reply] --> I[Success]
        H --> J[Error]
        H --> K[Finished]
    end
```

## 4. 容器配置

```mermaid
graph TB
    subgraph "Container Config"
        A[Config] --> B[WorkDir]
        A --> C[Mounts]
        A --> D[SymbolicLinks]
        A --> E[InitCommand]
        A --> F[Credentials]
    end
    
    subgraph "Resources"
        G[Limits] --> H[RLimits]
        G --> I[Seccomp]
    end
```

## 5. 进程状态

```mermaid
stateDiagram-v2
    [*] --> Init
    Init --> Ready: 配置完成
    Ready --> Running: 执行命令
    Running --> Ready: 命令完成
    Ready --> [*]: 退出
```

## 6. 核心组件说明

### 6.1 Container Server
```go
type containerServer struct {
    socket *socket           // Unix socket 通信
    containerConfig         // 容器配置
    defaultEnv []string     // 默认环境变量
    
    // 通信通道
    recvCh chan recvCmd     // 接收命令
    sendCh chan sendReply   // 发送响应
    
    // 进程管理
    waitPid chan int
    waitPidResult chan waitPidResult
}
```

### 6.2 Command 结构
```go
type cmd struct {
    DeleteCmd *deleteCmd    // 删除操作
    ExecCmd   *execCmd      // 执行程序
    ConfCmd   *confCmd      // 设置配置
    OpenCmd   []OpenCmd     // 打开文件
    Cmd       cmdType       // 命令类型
}
```

### 6.3 Reply 结构
```go
type reply struct {
    Error     *errorReply   // 错误信息
    ExecReply *execReply    // 执行结果
}
```

## 7. 安全特性

```mermaid
graph TB
    subgraph "Security"
        A[Isolation] --> B[Namespaces]
        A --> C[Seccomp]
        A --> D[Resource Limits]
    end
```
