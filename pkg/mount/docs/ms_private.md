# MS_PRIVATE 挂载标志详解

## 挂载传播机制

```mermaid
graph TD
    subgraph 默认行为
        H1[宿主机挂载点] --> C1[容器1挂载点]
        H1 --> C2[容器2挂载点]
        H1 -->|新挂载会传播| C1
        H1 -->|新挂载会传播| C2
    end
    
    subgraph MS_PRIVATE行为
        H2[宿主机挂载点] -.->|不传播| C3[容器1挂载点]
        H2 -.->|不传播| C4[容器2挂载点]
        H2 -.-x|阻止传播| C3
        H2 -.-x|阻止传播| C4
    end
    
    style H1,H2 fill:#f9f,stroke:#333
    style C1,C2,C3,C4 fill:#bfb,stroke:#333
```

## 工作原理

```mermaid
sequenceDiagram
    participant Host as 宿主机
    participant Kernel as 内核
    participant Container as 容器
    
    Note over Host,Container: 使用 MS_PRIVATE
    
    Host->>Kernel: 创建新挂载点
    Kernel->>Kernel: 检查传播标志
    Kernel-->>Container: 不传播挂载事件
    
    Container->>Kernel: 创建新挂载点
    Kernel->>Kernel: 检查传播标志
    Kernel-->>Host: 不传播挂载事件
```

## 四种挂载传播类型对比

```mermaid
graph LR
    subgraph 传播类型
        P[MS_PRIVATE]
        S[MS_SHARED]
        SL[MS_SLAVE]
        U[MS_UNBINDABLE]
    end
    
    P -->|完全隔离| PE[不共享挂载事件]
    S -->|双向传播| SE[共享所有挂载事件]
    SL -->|单向传播| SLE[只接收主挂载事件]
    U -->|禁止绑定| UE[禁止作为绑定源]
    
    classDef type fill:#f9f,stroke:#333
    classDef effect fill:#bfb,stroke:#333
    
    class P,S,SL,U type
    class PE,SE,SLE,UE effect
```

## 新挂载传播示例

假设我们有以下场景：
```bash
# 宿主机上的目录结构
/mnt/
    └── data/           # 初始挂载点
        └── file1.txt

# 容器中的目录结构（通过绑定挂载）
/container/mnt/
    └── data/          # 从宿主机挂载过来的
        └── file1.txt
```

### 1. 默认情况（没有 MS_PRIVATE）
```mermaid
sequenceDiagram
    participant Host as 宿主机
    participant Container as 容器
    
    Note over Host,Container: 初始状态：都能看到 /mnt/data
    
    Host->>Host: 挂载新设备到 /mnt/data/usb
    Note over Host: 新增 usb 目录和文件
    Host-->>Container: 自动传播！
    Note over Container: 容器也能看到 usb 目录
    
    Note over Host,Container: 最终两边都能看到：<br>/mnt/data/<br>  ├── file1.txt<br>  └── usb/
```

### 2. 使用 MS_PRIVATE
```mermaid
sequenceDiagram
    participant Host as 宿主机
    participant Container as 容器
    
    Note over Host,Container: 初始状态：都能看到 /mnt/data
    
    Host->>Host: 挂载新设备到 /mnt/data/usb
    Note over Host: 新增 usb 目录和文件
    Host--xContainer: 传播被阻止！
    Note over Container: 容器仍然只能看到原来的内容
    
    Note over Host: 宿主机看到：<br>/mnt/data/<br>  ├── file1.txt<br>  └── usb/
    Note over Container: 容器只看到：<br>/mnt/data/<br>  └── file1.txt
```

### 具体示例代码
```go
// 1. 默认情况（挂载会传播）
mount := &Mount{
    Source: "/mnt/data",
    Target: "/container/mnt/data",
    Flags:  unix.MS_BIND,
}
// 如果宿主机在 /mnt/data 下挂载了新设备
// 容器内的 /container/mnt/data 也会看到新设备

// 2. 使用 MS_PRIVATE（阻止传播）
mount := &Mount{
    Source: "/mnt/data",
    Target: "/container/mnt/data",
    Flags:  unix.MS_BIND | unix.MS_PRIVATE,
}
// 即使宿主机在 /mnt/data 下挂载了新设备
// 容器内的 /container/mnt/data 也不会看到变化
```

### 实际影响
1. **没有 MS_PRIVATE 时**：
   - 宿主机挂载 USB 设备：容器也能看到
   - 宿主机挂载新分区：容器也能看到
   - 宿主机挂载临时文件系统：容器也能看到

2. **使用 MS_PRIVATE 时**：
   - 宿主机的新挂载操作不会影响容器
   - 容器只能看到最初挂载时的内容
   - 提供了更好的隔离性和安全性

### 安全隐患
如果不使用 MS_PRIVATE：
1. 宿主机挂载敏感数据，容器可能意外看到
2. 宿主机的挂载操作可能影响容器运行
3. 容器间可能通过挂载点互相影响

## 实际应用示例

### 1. 基本使用
```go
// 创建私有挂载点
mount := &Mount{
    Source: "/source",
    Target: "/target",
    Flags:  unix.MS_BIND | unix.MS_PRIVATE,
}

// 在容器中使用
builder := mount.NewDefaultBuilder().
    WithMount(mount).
    WithPrivateOption()  // 确保所有挂载点都是私有的
```

### 2. 安全隔离场景

```mermaid
graph TD
    subgraph 容器环境
        subgraph 安全区域
            S1[敏感数据目录]
            S2[配置文件]
        end
        
        subgraph 普通区域
            N1[应用目录]
            N2[临时文件]
        end
    end
    
    S1 -->|MS_PRIVATE| I1[隔离的挂载点]
    S2 -->|MS_PRIVATE| I2[隔离的挂载点]
    
    style S1,S2 fill:#f99,stroke:#333
    style N1,N2 fill:#9f9,stroke:#333
    style I1,I2 fill:#99f,stroke:#333
```

## 使用场景

### 1. 容器隔离
```mermaid
mindmap
    root((MS_PRIVATE使用场景))
        敏感数据保护
            配置文件
            密钥存储
            用户数据
        容器隔离
            文件系统隔离
            挂载点隔离
            资源隔离
        安全加固
            防止信息泄露
            防止权限提升
            防止交叉访问
```

### 2. 具体示例
```go
// 敏感数据目录
mount := &Mount{
    Source: "/secrets",
    Target: "/container/secrets",
    Flags:  unix.MS_BIND | unix.MS_PRIVATE | unix.MS_RDONLY,
}

// 配置文件
mount := &Mount{
    Source: "/configs",
    Target: "/container/configs",
    Flags:  unix.MS_BIND | unix.MS_PRIVATE | unix.MS_RDONLY,
}
```

## 调试和验证

### 1. 检查挂载属性
```bash
# 查看挂载点属性
findmnt -o TARGET,PROPAGATION /path/to/mount

# 详细挂载信息
cat /proc/self/mountinfo | grep private
```

### 2. 验证隔离效果
```bash
# 在容器内
mount | grep "private"

# 在宿主机检查
nsenter -t <container_pid> -m -- findmnt
```

## 最佳实践

### 1. 安全建议
- 总是对敏感挂载点使用 MS_PRIVATE
- 组合使用 MS_RDONLY 提供额外保护
- 定期审计挂载点配置

### 2. 性能考虑
- MS_PRIVATE 不会带来明显性能开销
- 可以减少不必要的挂载事件传播
- 简化挂载点管理

### 3. 故障排除
```mermaid
graph TD
    P[问题发现] --> C1{挂载点可见性}
    C1 -->|可见| S1[检查MS_PRIVATE标志]
    C1 -->|不可见| S2[检查挂载配置]
    
    S1 --> A1[添加MS_PRIVATE]
    S2 --> A2[验证挂载参数]
    
    style P fill:#f99,stroke:#333
    style C1 fill:#99f,stroke:#333
    style S1,S2 fill:#9f9,stroke:#333
    style A1,A2 fill:#ff9,stroke:#333
```
