# Linux 命名空间详解

## 什么是命名空间？

命名空间是 Linux 内核提供的一种资源隔离机制，它可以让一组进程看到一个资源集合，而另一组进程看到另一个资源集合。

```mermaid
graph TD
    subgraph 物理系统
        R[系统资源]
        
        subgraph NS1[命名空间 1]
            P1[进程组 1]
            V1[资源视图 1]
        end
        
        subgraph NS2[命名空间 2]
            P2[进程组 2]
            V2[资源视图 2]
        end
        
        R --> V1
        R --> V2
    end
    
    style R fill:#f9f,stroke:#333
    style V1,V2 fill:#bfb,stroke:#333
    style P1,P2 fill:#ffd,stroke:#333
```

## 命名空间类型

```mermaid
mindmap
    root((Linux命名空间))
        Mount命名空间
            独立的挂载点视图
            文件系统隔离
            挂载传播控制
        PID命名空间
            进程ID隔离
            独立的进程树
            嵌套的PID空间
        Network命名空间
            网络设备隔离
            IP地址隔离
            路由表隔离
        UTS命名空间
            主机名隔离
            NIS域名隔离
        IPC命名空间
            消息队列隔离
            共享内存隔离
            信号量隔离
        User命名空间
            用户ID映射
            用户组ID映射
            特权隔离
```

## 命名空间的工作原理

```mermaid
sequenceDiagram
    participant P as 父进程
    participant K as 内核
    participant C as 子进程
    
    P->>K: clone() 系统调用
    Note over K: 创建新的命名空间
    K->>C: 在新命名空间中运行
    Note over C: 独立的资源视图
    
    C->>K: 资源操作请求
    K->>K: 检查命名空间
    K-->>C: 返回命名空间内的资源视图
```

## 命名空间的特性

### 1. 嵌套性
```mermaid
graph TD
    H[宿主机命名空间] --> C1[容器1命名空间]
    C1 --> C2[容器2命名空间]
    
    style H fill:#f9f,stroke:#333
    style C1 fill:#bfb,stroke:#333
    style C2 fill:#ffd,stroke:#333
```

### 2. 独立性
```mermaid
graph LR
    subgraph NS1[命名空间 1]
        R1[资源集合 1]
    end
    
    subgraph NS2[命名空间 2]
        R2[资源集合 2]
    end
    
    NS1 -.->|隔离| NS2
    
    style NS1,NS2 fill:#f9f,stroke:#333
    style R1,R2 fill:#bfb,stroke:#333
```

## 实际应用示例

### 1. 创建新的命名空间
```go
// 创建一个新的命名空间
cmd := exec.Command("bash")
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWNS |  // 新的挂载命名空间
               syscall.CLONE_NEWUTS |   // 新的UTS命名空间
               syscall.CLONE_NEWPID |   // 新的PID命名空间
               syscall.CLONE_NEWNET,    // 新的网络命名空间
}
```

### 2. 命名空间操作
```go
// 进入已存在的命名空间
cmd := exec.Command("nsenter", 
    "-t", pid,           // 目标进程
    "-m",               // 进入挂载命名空间
    "-u",               // 进入UTS命名空间
    "-p",               // 进入PID命名空间
    "-n",               // 进入网络命名空间
    "command")          // 要执行的命令
```

## 各类命名空间详解

### 1. Mount 命名空间
- 隔离文件系统挂载点
- 控制挂载点可见性
- 管理挂载传播

```mermaid
graph TD
    subgraph 宿主机
        H[根文件系统]
        
        subgraph Container1[容器1]
            M1[挂载点视图1]
        end
        
        subgraph Container2[容器2]
            M2[挂载点视图2]
        end
    end
    
    H --> M1
    H --> M2
    
    style H fill:#f9f,stroke:#333
    style M1,M2 fill:#bfb,stroke:#333
```

### 2. PID 命名空间
- 进程ID独立编号
- 父子关系隔离
- 信号处理隔离

```mermaid
graph TD
    subgraph Host[宿主机 PID空间]
        P1[PID 1: systemd]
        
        subgraph Container[容器 PID空间]
            CP1[PID 1: 容器进程]
            CP2[PID 2: 子进程]
        end
    end
    
    P1 --> CP1
    CP1 --> CP2
    
    style P1 fill:#f9f,stroke:#333
    style CP1,CP2 fill:#bfb,stroke:#333
```

### 3. Network 命名空间
- 网络设备隔离
- 地址空间隔离
- 路由规则隔离

```mermaid
graph LR
    subgraph Host[宿主机网络]
        N1[eth0]
        
        subgraph Container[容器网络]
            CN1[veth0]
        end
    end
    
    N1 <-->|虚拟网桥| CN1
    
    style N1 fill:#f9f,stroke:#333
    style CN1 fill:#bfb,stroke:#333
```

## 命名空间的应用场景

### 1. 容器化
- Docker/Podman
- LXC/LXD
- Kubernetes

### 2. 安全隔离
- 资源访问控制
- 权限分离
- 攻击面减少

### 3. 资源管理
- 资源限制
- 资源监控
- 资源分配

## 最佳实践

### 1. 安全考虑
- 最小权限原则
- 资源限制
- 监控和审计

### 2. 性能优化
- 合理使用共享
- 避免过度隔离
- 资源效率

### 3. 调试技巧
```bash
# 查看进程的命名空间
ls -l /proc/<pid>/ns/

# 查看挂载信息
cat /proc/<pid>/mountinfo

# 查看网络命名空间
ip netns list
```
