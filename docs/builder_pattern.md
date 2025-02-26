# Builder 模式及其在 GoSandbox 中的应用

## 1. Builder 模式概述

### 1.1 定义
Builder 模式是一种创建型设计模式，它将一个复杂对象的构建过程与其表示分离，使得同样的构建过程可以创建不同的表示。这种模式特别适用于构建具有多个配置选项的复杂对象。

### 1.2 核心组件
- **Director**：指导构建过程，但不直接创建最终产品
- **Builder 接口**：定义创建产品各个部分的抽象方法
- **具体 Builder**：实现 Builder 接口，构建和装配各个部分
- **Product**：最终要创建的复杂对象

### 1.3 优势
1. **分步构建**：将对象构建过程分解为多个步骤
2. **参数灵活性**：支持多种可选参数，避免构造函数参数过多
3. **代码可读性**：通过方法链（method chaining）提高代码可读性
4. **不变性**：可以创建不可变对象
5. **封装复杂性**：隐藏产品内部表示和构建过程

## 2. Go 语言中的 Builder 模式

Go 语言中的 Builder 模式通常有两种实现方式：

### 2.1 函数选项模式
```go
type Options struct {
    // 配置字段
}

type Option func(*Options)

func WithField1(value string) Option {
    return func(o *Options) {
        o.Field1 = value
    }
}

func NewObject(opts ...Option) *Object {
    options := &Options{
        // 默认值
    }
    
    for _, opt := range opts {
        opt(options)
    }
    
    // 使用 options 创建对象
}
```

### 2.2 方法链式 Builder
```go
type Builder struct {
    // 构建字段
}

func NewBuilder() *Builder {
    return &Builder{
        // 默认值
    }
}

func (b *Builder) WithField1(value string) *Builder {
    b.field1 = value
    return b
}

func (b *Builder) Build() (Product, error) {
    // 验证和构建逻辑
    return Product{...}, nil
}
```

## 3. GoSandbox 中的 Builder 模式应用

GoSandbox 项目中，Builder 模式主要应用在容器创建和配置过程中。以下是几个关键示例：

### 3.1 container.Builder

`container.Builder` 是项目中最典型的 Builder 模式应用，用于创建和配置容器环境：

```go
// container/environment_linux.go
type Builder struct {
    Root        string
    WorkDir     string
    HostName    string
    DomainName  string
    Mounts      []Mount
    Devices     []Device
    Env         []string
    UidMappings []IDMapping
    GidMappings []IDMapping
    // 其他配置字段...
}

func (b *Builder) Build() (Environment, error) {
    // 验证配置
    if b.Root == "" {
        return nil, errors.New("root must be set")
    }
    
    // 创建容器
    c, err := b.startContainer()
    if err != nil {
        return nil, err
    }
    
    // 返回环境接口
    return c, nil
}
```

使用示例：
```go
builder := &container.Builder{
    Root:     "/sandbox/root",
    WorkDir:  "/w",
    HostName: "sandbox",
    Mounts: []container.Mount{
        {Source: "/lib", Target: "/lib", Flags: syscall.MS_BIND | syscall.MS_REC},
    },
    Env: []string{"PATH=/usr/bin"},
}

env, err := builder.Build()
if err != nil {
    // 处理错误
}
```

### 3.2 seccomp.Builder

`seccomp.Builder` 用于构建安全计算（seccomp）过滤器，控制容器内可执行的系统调用：

```go
// seccomp/seccomp_linux.go
type Builder struct {
    defaultAction Action
    rules         []Rule
}

func NewBuilder(defaultAction Action) *Builder {
    return &Builder{
        defaultAction: defaultAction,
    }
}

func (b *Builder) AddRule(syscall uintptr, action Action) *Builder {
    b.rules = append(b.rules, Rule{Syscall: syscall, Action: action})
    return b
}

func (b *Builder) Build() (*Filter, error) {
    // 构建 seccomp 过滤器
    return &Filter{
        defaultAction: b.defaultAction,
        rules:         b.rules,
    }, nil
}
```

使用示例：
```go
seccompBuilder := seccomp.NewBuilder(seccomp.ActionKill)
seccompBuilder.AddRule(syscall.SYS_READ, seccomp.ActionAllow)
seccompBuilder.AddRule(syscall.SYS_WRITE, seccomp.ActionAllow)

filter, err := seccompBuilder.Build()
if err != nil {
    // 处理错误
}
```

### 3.3 runner.Builder

`runner.Builder` 用于构建和配置运行器，负责执行程序：

```go
// runner/builder.go
type Builder struct {
    execFile    string
    args        []string
    env         []string
    workDir     string
    files       []uintptr
    rlimits     []rlimit.RLimit
    // 其他配置...
}

func NewBuilder() *Builder {
    return &Builder{
        // 默认值
    }
}

func (b *Builder) WithExecFile(execFile string) *Builder {
    b.execFile = execFile
    return b
}

func (b *Builder) WithArgs(args []string) *Builder {
    b.args = args
    return b
}

func (b *Builder) Build() (Runner, error) {
    // 验证配置
    if b.execFile == "" {
        return nil, errors.New("exec file must be set")
    }
    
    // 根据配置创建合适的 Runner 实现
    return &concreteRunner{
        execFile: b.execFile,
        args:     b.args,
        env:      b.env,
        // 其他字段...
    }, nil
}
```

## 4. Builder 模式的实际优势

在 GoSandbox 项目中，Builder 模式带来了以下具体优势：

### 4.1 处理复杂配置
容器创建涉及大量配置项（挂载点、环境变量、资源限制等），Builder 模式使这些配置更易于管理。

### 4.2 提供合理默认值
各种 Builder 可以为非必要参数提供合理的默认值，简化客户端代码。

### 4.3 参数验证
在 `Build()` 方法中集中进行参数验证，确保创建有效的对象。

### 4.4 封装创建逻辑
复杂的容器创建和初始化逻辑被封装在 Builder 内部，对外提供简洁的接口。

### 4.5 支持不同配置组合
允许根据不同需求灵活组合配置选项，创建各种容器环境。

## 5. 容器池化中的应用

在容器池化实现中，Builder 模式发挥了关键作用：

```go
// 创建容器池
func createContainerPool(size int) ([]*container.Environment, error) {
    var pool []*container.Environment
    
    // 创建 Builder 实例
    builder := &container.Builder{
        Root:      "/sandbox/root",
        WorkDir:   "/w",
        HostName:  "sandbox",
        // 共享配置...
    }
    
    // 使用同一个 Builder 创建多个容器
    for i := 0; i < size; i++ {
        env, err := builder.Build()
        if err != nil {
            // 清理已创建的容器
            for _, e := range pool {
                e.Close()
            }
            return nil, err
        }
        pool = append(pool, env)
    }
    
    return pool, nil
}
```

这种方式确保了池中的所有容器具有一致的配置，同时允许在需要时轻松修改共享配置。

## 6. 最佳实践

基于 GoSandbox 项目的经验，以下是在 Go 中使用 Builder 模式的最佳实践：

### 6.1 明确必要参数
- 在 Builder 构造函数中要求提供必要参数
- 可选参数通过 With* 方法设置

### 6.2 方法链设计
- 每个设置方法返回 Builder 自身，支持链式调用
- 使用指针接收器以避免不必要的复制

### 6.3 验证逻辑集中化
- 在 Build() 方法中集中进行参数验证
- 提供有意义的错误信息

### 6.4 不可变性考虑
- 考虑让构建的对象不可变
- Builder 可以是可变的，但产品应该是不可变的

### 6.5 接口与实现分离
- Builder 返回接口类型而非具体实现
- 这增强了代码的灵活性和可测试性

## 7. 总结

Builder 模式在 GoSandbox 项目中的应用展示了它在处理复杂对象创建时的强大能力。通过分离对象的构建过程与表示，该模式使代码更具可读性、可维护性和灵活性。

在容器创建、安全过滤器配置和运行器设置等场景中，Builder 模式帮助开发者管理复杂配置，提供了清晰的 API，并确保了对象创建的正确性。这种模式特别适合 Go 语言的设计理念，与其简洁、实用的特性相得益彰。
