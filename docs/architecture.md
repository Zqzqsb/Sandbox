# Reproduce Sandbox 架构设计

## 1. 项目结构

```mermaid
graph TB
    subgraph "核心接口层"
        A[Runner Interface] --> B[运行结果]
        A --> C[资源限制]
    end
    
    subgraph "基础设施层"
        D[pkg/forkexec] --> J[进程创建与控制]
        E[pkg/seccomp] --> K[系统调用过滤]
        F[pkg/mount] --> L[文件系统隔离]
        G[pkg/cgroup] --> M[资源限制]
        H[pkg/rlimit] --> M
        I[pkg/unixsocket] --> N[进程间通信]
    end
    
    subgraph "沙箱实现层"
        O[ptracer] --> P[系统调用跟踪]
        Q[runner/ptrace] --> O
        R[runner/unshare] --> S[命名空间隔离]
        T[container] --> U[完整容器环境]
    end
    
    Q & R & T --> A
    
    style A fill:#f9f,stroke:#333,stroke-width:4px
    style D fill:#bbf,stroke:#333,stroke-width:2px
    style E fill:#bbf,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
```

## 2. 核心组件关系

```mermaid
graph TB
    subgraph "基础功能"
        A[forkexec] --> B[进程创建]
        C[seccomp] --> D[系统调用过滤]
        E[mount] --> F[文件系统]
        G[cgroup/rlimit] --> H[资源限制]
    end
    
    subgraph "ptracer"
        I[Tracer] --> J[系统调用跟踪]
        J --> K[文件访问控制]
        J --> L[资源使用统计]
    end
    
    subgraph "沙箱实现"
        M[ptrace] --> I
        M --> A & C & G
        
        N[unshare] --> A
        N --> C & E & G
        
        O[container] --> P[Unix Socket]
        O --> M & N
    end
    
    style I fill:#f9f,stroke:#333,stroke-width:2px
    style M fill:#bbf,stroke:#333,stroke-width:2px
    style N fill:#bbf,stroke:#333,stroke-width:2px
    style O fill:#bbf,stroke:#333,stroke-width:2px
```

## 3. 沙箱实现层次

```mermaid
graph TB
    subgraph "Container"
        A[Container 沙箱] --> B[Unix Socket]
        B --> C[进程间通信]
        A --> D[集成其他沙箱特性]
    end
    
    subgraph "Ptrace"
        E[Ptrace 沙箱] --> F[系统调用跟踪]
        F --> G[文件访问控制]
        F --> H[资源统计]
    end
    
    subgraph "Unshare"
        I[Unshare 沙箱] --> J[命名空间隔离]
        J --> K[文件系统隔离]
        J --> L[进程隔离]
    end
    
    A --> E
    A --> I
    
    style A fill:#f9f,stroke:#333,stroke-width:2px
    style E fill:#bbf,stroke:#333,stroke-width:2px
    style I fill:#bbf,stroke:#333,stroke-width:2px
```

## 4. 核心组件说明

### 4.1 基础设施层
1. **pkg/forkexec**：进程创建和控制
   - 提供类似 `syscall.ForkExec` 的功能
   - 支持更多的安全特性和隔离机制

2. **pkg/seccomp**：系统调用过滤
   - 定义系统调用过滤规则
   - 实现系统调用的精细控制

3. **pkg/mount**：文件系统隔离
   - 管理挂载点
   - 实现文件系统隔离

4. **pkg/cgroup & pkg/rlimit**：资源限制
   - CPU、内存等资源限制
   - 进程资源使用控制

### 4.2 ptracer 包
1. **核心功能**：
   - 实现系统调用跟踪
   - 提供底层的 ptrace 操作
   - 支持文件访问控制

2. **主要组件**：
   ```go
   type Tracer struct {
       Handler  // 系统调用处理器
       Runner   // 进程运行器
       Limit    // 资源限制
   }
   ```

### 4.3 沙箱实现
1. **Ptrace 沙箱**：
   ```go
   type Runner struct {
       Args []string      // 命令行参数
       Env  []string      // 环境变量
       Files []uintptr    // 文件描述符
       RLimits []RLimit   // 资源限制
       Seccomp Filter     // 系统调用过滤器
   }
   ```

2. **Unshare 沙箱**：
   ```go
   type Runner struct {
       Args []string
       Env  []string
       Root string        // 根目录
       Mounts []Mount     // 挂载点
       Seccomp Filter     // 系统调用过滤器
   }
   ```

3. **Container 沙箱**：
   ```go
   type container struct {
       socket *socket     // Unix Socket
       process *os.Process
       config containerConfig
   }
   ```

## 5. 执行流程

```mermaid
sequenceDiagram
    participant Client
    participant Runner
    participant Sandbox
    participant OS
    
    Client->>Runner: Run(context)
    Runner->>Sandbox: 创建沙箱环境
    
    alt Container
        Sandbox->>OS: 创建容器进程
        Sandbox->>OS: 建立 Socket 通信
        Sandbox->>OS: 应用 Ptrace 和 Unshare
    else Ptrace
        Sandbox->>OS: 创建被跟踪进程
        Sandbox->>OS: 跟踪系统调用
    else Unshare
        Sandbox->>OS: 创建隔离进程
        Sandbox->>OS: 设置命名空间
    end
    
    OS-->>Sandbox: 执行结果
    Sandbox-->>Runner: 资源统计
    Runner-->>Client: 运行结果
```

## 6. 特性对比

| 特性 | Ptrace | Unshare | Container |
|------|---------|----------|-----------|
| 隔离级别 | 系统调用 | 命名空间 | 完整 |
| 资源控制 | 基础 | 基础 | 完整 |
| 性能开销 | 高 | 低 | 中 |
| 安全级别 | 高 | 中 | 高 |
| 实现复杂度 | 中 | 低 | 高 |
| 调试能力 | 强 | 弱 | 强 |
| 通信机制 | 进程跟踪 | 无 | Socket |
| 依赖关系 | 独立 | 独立 | 依赖其他沙箱 |
