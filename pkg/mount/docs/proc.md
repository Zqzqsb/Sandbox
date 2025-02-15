# Proc 文件系统

## 概述

proc 是一个虚拟文件系统，提供了访问内核数据结构、进程信息和其他系统信息的接口。在沙箱环境中，proc 文件系统对于进程监控和资源管理至关重要。

## 架构设计

```mermaid
graph TD
    subgraph ProcFS["Proc 文件系统结构"]
        proc["proc"]
        proc_info["进程信息"]
        sys_info["系统信息"]
        kernel_param["内核参数"]
        
        proc --> proc_info
        proc --> sys_info
        proc --> kernel_param
        
        proc_info --> pid["PID目录"]
        proc_info --> self["self"]
        
        pid --> status["status"]
        pid --> fd["fd"]
        pid --> maps["maps"]
        pid --> limits["limits"]
        
        sys_info --> cpuinfo["cpuinfo"]
        sys_info --> meminfo["meminfo"]
        sys_info --> mounts["mounts"]
        
        kernel_param --> sysctl["sysctl"]
        kernel_param --> sysrq["sysrq"]
    end

    classDef root fill:#f9f,stroke:#333,stroke-width:2px
    classDef group fill:#bbf,stroke:#333
    classDef sysgroup fill:#bfb,stroke:#333
    classDef kernelgroup fill:#fbb,stroke:#333
    
    class proc root
    class proc_info group
    class sys_info sysgroup
    class kernel_param kernelgroup
```

## 在沙箱中的应用

### 1. 挂载配置
```go
// 只读挂载
builder.WithProc()

// 可写挂载（谨慎使用）
builder.WithProcRW(true)
```

### 2. 安全限制
```mermaid
flowchart LR
    subgraph Security["安全标志"]
        nosuid["MS_NOSUID\n禁用SUID"]
        nodev["MS_NODEV\n禁用设备"]
        noexec["MS_NOEXEC\n禁用执行"]
        rdonly["MS_RDONLY\n只读模式"]
    end
    
    builder["Builder"] --> nosuid
    builder --> nodev
    builder --> noexec
    builder --> rdonly
    
    classDef flag fill:#f99,stroke:#333
    class nosuid,nodev,noexec,rdonly flag
```

## 关键文件和目录

### 1. 进程相关
| 路径 | 描述 | 安全考虑 |
|------|------|----------|
| /proc/[pid]/status | 进程状态信息 | 只读访问 |
| /proc/[pid]/fd | 文件描述符 | 限制访问 |
| /proc/[pid]/maps | 内存映射 | 可能泄露信息 |
| /proc/[pid]/limits | 资源限制 | 只读访问 |

### 2. 系统信息
| 路径 | 描述 | 沙箱中的处理 |
|------|------|--------------|
| /proc/cpuinfo | CPU信息 | 可访问 |
| /proc/meminfo | 内存信息 | 可访问 |
| /proc/mounts | 挂载信息 | 需过滤敏感信息 |

## 性能监控

```mermaid
sequenceDiagram
    participant Sandbox as 沙箱控制器
    participant Proc as Proc文件系统
    participant Container as 容器进程
    
    Sandbox->>Proc: 挂载proc（只读）
    Sandbox->>Container: 启动进程
    
    loop 监控循环
        Sandbox->>Proc: 读取 /proc/[pid]/status
        Proc-->>Sandbox: 返回进程状态
        Sandbox->>Proc: 读取 /proc/[pid]/limits
        Proc-->>Sandbox: 返回资源限制
        
        alt 检测到异常
            Sandbox->>Container: 发送信号
            Container-->>Sandbox: 进程终止
        end
    end
```

## 最佳实践

### 1. 安全配置
- 默认使用只读挂载
- 启用所有限制性标志
- 过滤敏感信息

### 2. 资源监控
- 定期检查进程状态
- 监控资源使用
- 设置合理的限制

### 3. 错误处理
- 处理挂载失败
- 监控异常状态
- 实现优雅降级

## 故障排除

### 1. 常见问题
1. **挂载失败**
   - 检查权限
   - 验证内核支持
   - 确认标志位组合

2. **权限问题**
   - 验证 UID/GID 映射
   - 检查安全标志
   - 确认访问权限

3. **性能问题**
   - 减少读取频率
   - 优化监控逻辑
   - 使用缓存机制

### 2. 调试技巧
```bash
# 检查挂载状态
mount | grep proc

# 查看进程信息
cat /proc/[pid]/status

# 检查资源限制
cat /proc/[pid]/limits
```

## 安全注意事项

### 1. 信息泄露
- 过滤敏感的内存映射
- 限制文件描述符访问
- 控制系统信息可见性

### 2. 权限控制
- 禁用特权操作
- 限制写入操作
- 实施访问控制

### 3. 资源保护
- 设置资源限制
- 监控资源使用
- 实现自动清理
