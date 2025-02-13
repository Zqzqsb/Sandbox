# libseccomp 包设计文档

## 概述

`libseccomp` 包是 `seccomp` 包的一个具体实现，它基于 [elastic/go-seccomp-bpf](https://github.com/elastic/go-seccomp-bpf) 库提供了 seccomp 过滤器的生成功能。该包采用 Builder 模式，提供了简单的接口来创建复杂的系统调用过滤规则。

## 包结构

```
libseccomp/
├── action.go          # 定义动作类型和常量
├── builder_linux.go   # 实现过滤器构建器
└── seccomp_linux_test.go  # 测试文件
```

## 核心组件

### 1. Action 类型

`Action` 是一个 32 位无符号整数，用于定义系统调用的处理动作：
- 低 16 位：基本动作（如 ALLOW、KILL 等）
- 高 16 位：附加数据（如错误码、跟踪标志等）

支持的动作类型：
- `ActionAllow`: 允许系统调用继续执行
- `ActionErrno`: 返回一个错误码给调用进程
- `ActionTrace`: 通知跟踪器并暂停执行
- `ActionKill`: 立即终止进程

### 2. Builder 结构体

Builder 采用构建器模式，用于创建 seccomp 过滤器：

```go
type Builder struct {
    Allow []string   // 允许执行的系统调用列表
    Trace []string   // 需要跟踪的系统调用列表
    Default Action   // 默认动作
}
```

主要方法：
- `Build()`: 将配置转换为可执行的 BPF 过滤器

### 3. 过滤器生成流程

1. 创建过滤策略：
   - 设置默认动作
   - 配置允许的系统调用
   - 配置需要跟踪的系统调用

2. 编译为 BPF 程序：
   - 使用 go-seccomp-bpf 库将策略转换为 BPF 指令
   - BPF 指令是一种在内核空间执行的虚拟机指令集

3. 转换为内核可读格式：
   - 将 BPF 指令转换为 SockFilter 格式
   - SockFilter 包含：操作码、跳转目标、立即数/地址

## 使用示例

```go
// 创建过滤器构建器
builder := libseccomp.Builder{
    Allow: []string{"read", "write"},  // 允许的系统调用
    Trace: []string{"open", "execve"}, // 需要跟踪的系统调用
    Default: libseccomp.ActionKill,    // 默认动作：终止进程
}

// 构建过滤器
filter, err := builder.Build()
if err != nil {
    // 处理错误
}
```

## 与 seccomp 包的关系

1. `libseccomp` 包是 `seccomp` 包的一个具体实现
2. `seccomp` 包定义了基本的接口和类型（如 Filter 类型）
3. `libseccomp` 包实现了这些接口，并提供了额外的功能

## 性能考虑

1. 使用全局变量缓存常用动作，避免重复创建
2. 过滤器构建性能约为 0.2ms/op
3. 生成的 BPF 过滤器在内核空间高效执行

## 安全考虑

1. 默认采用白名单方式：只允许显式指定的系统调用
2. 提供多种处理动作：允许、拒绝、跟踪、终止
3. 支持细粒度的系统调用控制
