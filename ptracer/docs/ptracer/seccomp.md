# Seccomp (Secure Computing Mode)

## 1. 基本概念

seccomp 是 Linux 内核提供的安全机制，用于限制进程可以使用的系统调用。

### 1.1 工作模式

1. strict 模式：
```c
// 只允许 4 个系统调用
- read()
- write()
- exit()
- sigreturn()
```

2. filter 模式：
```c
// 使用 BPF 规则定义
- 可以自定义允许/禁止的系统调用
- 可以设置违规行为
- 支持参数过滤
```

### 1.2 过滤规则

使用 BPF (Berkeley Packet Filter) 语法：
```c
struct sock_filter {
    __u16 code;    // 操作码
    __u8  jt;      // true 跳转
    __u8  jf;      // false 跳转
    __u32 k;       // 通用字段
};
```

示例规则：
```c
// 允许 open 系统调用
BPF_STMT(BPF_LD+BPF_W+BPF_ABS, syscall_nr)
BPF_JUMP(BPF_JMP+BPF_JEQ+BPF_K, __NR_open, 0, 1)
BPF_STMT(BPF_RET+BPF_K, SECCOMP_RET_ALLOW)
BPF_STMT(BPF_RET+BPF_K, SECCOMP_RET_KILL)
```

## 2. 在沙箱中的应用

### 2.1 基本流程

1. 设置 seccomp：
```go
// 1. 创建 filter
filter := seccomp.NewFilter()

// 2. 添加规则
filter.AddRule(unix.SYS_OPEN, 
    seccomp.Allow)

// 3. 加载规则
filter.Load()
```

2. 与 ptrace 配合：
```go
// 1. 设置 ptrace 选项
unix.PtraceSetOptions(pid,
    unix.PTRACE_O_TRACESECCOMP)

// 2. 处理 seccomp 事件
if event == unix.PTRACE_EVENT_SECCOMP {
    handleSeccompEvent()
}
```

### 2.2 处理策略

1. 直接动作：
```c
SECCOMP_RET_ALLOW  // 允许系统调用
SECCOMP_RET_KILL   // 终止进程
SECCOMP_RET_TRAP   // 产生 SIGSYS
SECCOMP_RET_ERRNO  // 返回错误码
```

2. 通知动作：
```c
SECCOMP_RET_TRACE  // 通知 tracer
- 允许 tracer 检查和修改
- 可以动态决定处理方式
- 支持复杂的控制逻辑
```

## 3. 实现细节

### 3.1 系统调用处理

```go
func (ph *ptraceHandle) handleTrap(pid int) error {
    // 1. 获取系统调用上下文
    ctx := getContext(pid)
    
    // 2. 调用处理器
    action := ph.Handler.Handle(ctx)
    
    // 3. 执行相应动作
    switch action {
    case TraceBan:
        // 跳过系统调用
        skipSyscall(pid)
    case TraceKill:
        // 终止进程
        killProcess(pid)
    case TraceAllow:
        // 允许继续
        return nil
    }
}
```

### 3.2 常见限制场景

1. 文件操作：
```go
// 限制文件访问
- open()
- read()
- write()
- unlink()
```

2. 进程控制：
```go
// 禁止创建新进程
- fork()
- clone()
- execve()
```

3. 网络访问：
```go
// 限制网络操作
- socket()
- connect()
- bind()
```

## 4. 安全考虑

### 4.1 注意事项

1. 规则顺序：
```go
// 规则按顺序匹配
- 更具体的规则放前面
- 默认规则放最后
```

2. 参数检查：
```go
// 检查系统调用参数
- 文件路径
- 权限标志
- 网络地址
```

3. 错误处理：
```go
// 合理处理违规行为
- 日志记录
- 错误报告
- 进程清理
```

### 4.2 最佳实践

1. 最小权限：
```go
// 只允许必需的系统调用
- 分析程序需求
- 逐个添加允许
- 经常性审查
```

2. 动态控制：
```go
// 使用 SECCOMP_RET_TRACE
- 运行时决策
- 上下文感知
- 灵活处理
```

3. 防逃逸：
```go
// 防止绕过限制
- 禁止危险系统调用
- 检查参数组合
- 监控异常行为
```

## 5. 调试技巧

### 5.1 常用工具

1. strace：
```bash
# 跟踪系统调用
strace -f ./program
```

2. seccomp-tools：
```bash
# 分析 seccomp 规则
seccomp-tools dump ./program
```

### 5.2 故障排除

1. 系统调用失败：
```
- 检查规则配置
- 查看系统日志
- 分析错误码
```

2. 进程异常终止：
```
- 确认所需系统调用
- 检查参数限制
- 调整处理策略
```
