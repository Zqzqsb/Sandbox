# forkexec 包结构说明

## 1. 包的核心功能

forkexec 包的主要目标是：**在 Linux 系统上安全地创建和运行隔离的进程**

```mermaid
graph TD
    A[创建隔离进程] --> B[资源隔离]
    A --> C[安全控制]
    A --> D[进程管理]
    
    B --> B1[命名空间隔离]
    B --> B2[资源限制]
    B --> B3[文件系统隔离]
    
    C --> C1[权限控制]
    C --> C2[系统调用过滤]
    
    D --> D1[进程创建]
    D --> D2[进程执行]
    D --> D3[进程监控]
```

## 2. 文件组织

### 2.1 核心文件及其关系
```mermaid
graph LR
    R[runner_linux.go<br>进程运行器] --> F[fork_linux.go<br>进程创建]
    F --> C[fork_child_linux.go<br>子进程执行]
    
    subgraph 辅助功能
        U[userns_linux.go<br>用户空间]
        E[errloc_linux.go<br>错误处理]
        T[fork_util.go<br>工具函数]
    end
    
    F --> U
    F --> E
    F --> T
```

### 2.2 文件功能说明

1. **runner_linux.go**：进程运行的控制中心
   - 配置进程运行环境
   - 管理资源限制
   - 控制进程生命周期

2. **fork_linux.go**：进程创建的核心实现
   - 实现 fork 操作
   - 设置命名空间隔离
   - 管理子进程通信

3. **fork_child_linux.go**：子进程的具体实现
   - 配置执行环境
   - 应用安全限制
   - 执行目标程序

4. **辅助文件**：
   - `userns_linux.go`：用户命名空间配置
   - `errloc_linux.go`：错误定位和处理
   - `fork_util.go`：通用工具函数

## 3. 执行流程

```mermaid
sequenceDiagram
    participant App as 应用程序
    participant Runner as 运行器
    participant Fork as 进程创建
    participant Child as 子进程
    
    App->>Runner: 1. 配置运行参数
    Note over Runner: 设置资源限制<br>配置安全选项
    
    Runner->>Fork: 2. 创建进程
    Note over Fork: 创建命名空间<br>设置隔离环境
    
    Fork->>Child: 3. 启动子进程
    Note over Child: 应用限制<br>执行程序
    
    Child-->>Fork: 4. 执行状态
    Fork-->>Runner: 5. 进程信息
    Runner-->>App: 6. 执行结果
```

## 4. 关键功能点

### 4.1 资源隔离
- 命名空间隔离（PID、网络、挂载等）
- 资源限制（CPU、内存、文件等）
- 文件系统隔离（pivot_root）

### 4.2 安全机制
- seccomp 系统调用过滤
- 用户空间隔离
- 权限控制

### 4.3 进程管理
- 进程创建和执行
- 资源清理
- 错误处理

## 5. 使用示例

```go
runner := &Runner{
    // 基本配置
    Args: []string{"/bin/program"},
    Env:  []string{"PATH=/bin"},
    
    // 资源限制
    RLimits: []rlimit.RLimit{...},
    
    // 安全选项
    NoNewPrivs: true,
    Seccomp:    seccompFilter,
    
    // 隔离配置
    CloneFlags: unix.CLONE_NEWPID | unix.CLONE_NEWNS,
}

// 执行程序
err := runner.Start()