# CMD 包架构设计

## 1. 定位和作用

`cmd/runprog` 是一个用于测试和验证沙箱功能的命令行工具，它不是核心功能的一部分，而是帮助我们开发和测试的辅助工具。

```mermaid
graph TB
    subgraph "reproduce_sanbox"
        subgraph "沙箱实现"
            A[container] --> D[沙箱接口]
            B[ptrace] --> D
            C[unshare] --> D
        end
        
        subgraph "cmd/runprog"
            E[命令行工具] --> F[参数解析]
            F --> G[沙箱选择]
            G --> D
        end
    end
    
    style D fill:#f9f,stroke:#333,stroke-width:2px
    style E fill:#bbf,stroke:#333,stroke-width:2px
```

### 1.1 主要功能
- 提供统一的命令行参数
- 选择不同的沙箱实现
- 设置运行限制（时间、内存等）
- 输出运行结果

### 1.2 依赖关系
```
cmd/runprog
├── 依赖 container 包
├── 依赖 ptrace 包
├── 依赖 unshare 包
└── 依赖 runner 接口
```

## 2. 目录结构

```
cmd/
└── runprog/                # 程序运行器
    ├── config/            # 配置文件
    ├── array_flags.go     # 命令行数组标志
    ├── fileutil.go        # 文件工具
    ├── main.go            # 主程序入口
    ├── main_darwin.go     # MacOS 实现
    └── main_linux.go      # Linux 实现
```

## 3. 运行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| -tl | 时间限制（秒） | 1 |
| -rtl | 实际时间限制（秒） | 0 |
| -ml | 内存限制（MB） | 256 |
| --allow-proc | 允许访问 /proc | false |
| --unsafe | 不安全模式 | false |
| --show-details | 显示详细信息 | false |
| --cgroup | 使用 cgroup | false |
| --cred | 使用凭证 | false |

## 4. 状态定义

```mermaid
graph LR
    subgraph "运行状态"
        A[Status] --> B[Normal]
        A --> C[Invalid]
        A --> D[RE]
        A --> E[MLE]
        A --> F[TLE]
        A --> G[OLE]
        A --> H[Ban]
        A --> I[Fatal]
    end
    
    style A fill:#f9f,stroke:#333,stroke-width:2px
```

## 5. 执行流程

```mermaid
sequenceDiagram
    participant Main as Main
    participant Runner as Runner
    participant Sandbox as Sandbox
    
    Main->>Runner: 解析参数
    Main->>Runner: 选择运行环境
    Runner->>Sandbox: 创建沙箱
    Sandbox->>Sandbox: 设置限制
    Sandbox->>Runner: 运行程序
    Runner->>Main: 返回结果
```

## 6. 核心组件

### 6.1 主程序
```go
func main() {
    // 解析命令行参数
    flag.Uint64Var(&timeLimit, "tl", 1, "时间限制")
    flag.Uint64Var(&memoryLimit, "ml", 256, "内存限制")
    // ...

    // 选择运行环境
    var r runner.Runner
    switch pType {
    case "container":
        r = container环境
    case "ptrace":
        r = ptrace环境
    case "unshare":
        r = unshare环境
    }

    // 运行程序
    result := r.Run(context)
}
```

### 6.2 状态映射
```go
func getStatus(s runner.Status) int {
    switch s {
    case runner.StatusNormal:
        return StatusNormal
    case runner.StatusTimeLimitExceeded:
        return StatusTLE
    case runner.StatusMemoryLimitExceeded:
        return StatusMLE
    // ...
    }
}
```

## 7. 使用示例

```bash
# 基本使用
runprog -tl 1 -ml 256 ./program

# 带输入输出
runprog -i input.txt -o output.txt ./program

# 容器模式
runprog --type container ./program

# 详细信息
runprog --show-details ./program
