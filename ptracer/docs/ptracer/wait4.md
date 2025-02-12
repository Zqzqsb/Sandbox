# 进程跟踪与进程组管理

## 进程组的演变

### 1. 初始状态
```
Shell (PID=100, PGID=100)
  └── 沙箱进程 (PID=200, PGID=100)
      └── 目标程序 (PID=300, PGID=100)  // 刚创建，还未exec
```

### 2. 设置新进程组后
```
Shell (PID=100, PGID=100)           进程层次关系不变，但进程组已分离：
  └── 沙箱进程 (PID=200, PGID=100)  
      └── 目标程序 (PID=300, PGID=300) ┐ 新进程组
          ├── 子进程1 (PID=301, PGID=300) ├─ 共享PGID=300
          └── 子进程2 (PID=302, PGID=300) ┘
```

## PID 和 PGID 的独立分配机制

### 1. 基本特性
```
PID:
- 系统全局唯一，按序分配
- 进程终止后可重用
- 子进程获得新的PID

PGID:
- 默认继承父进程的PGID
- 可通过setpgid显式设置
- 多个PID可以共享同一PGID
```

### 2. 进程组关系
```
特点：
1. 进程层次关系（父子关系）不变
2. 进程组关系可以独立变化
3. 子进程默认继承父进程的PGID
4. 可以创建新的进程组
```

### 3. 进程组的作用
```
1. 信号传递：
   kill(-pgid, signal)  // 发送到整个组

2. 资源控制：
   - CPU限制
   - 内存限制
   - 文件限制

3. 作业控制：
   - 前台/后台运行
   - 终端控制
```

## Wait4 的工作机制

### 1. 参数说明
```c
// wait4 等待子进程的状态变化（退出、终止、停止）
pid_t wait4(pid_t pid, int *status, int options, struct rusage *rusage);

// pid 参数决定等待哪些子进程的状态变化：
pid < -1  : 等待进程组ID为|pid|的任何子进程的状态变化
pid = -1  : 等待任何子进程的状态变化
pid = 0   : 等待与调用进程同组的任何子进程的状态变化
pid > 0   : 等待指定PID的子进程的状态变化

// status 返回子进程的详细状态：
WIFEXITED(status)    : 子进程正常退出
WIFSIGNALED(status)  : 子进程被信号终止
WIFSTOPPED(status)   : 子进程被信号停止
WIFCONTINUED(status) : 子进程从停止状态继续运行

// options 控制等待行为：
WNOHANG    : 如果没有子进程状态改变，立即返回
WUNTRACED  : 报告被停止的子进程
WCONTINUED : 报告从停止状态继续的子进程
WALL       : 等待所有子进程

// rusage 返回子进程的资源使用统计：
- CPU时间
- 内存使用
- IO操作次数
- 上下文切换次数
等
```

### 2. 在沙箱中的应用
```go
// 1. exec前：精确等待特定进程
pid, err = unix.Wait4(pgid, &wstatus, unix.WALL, &rusage)

// 2. exec后：等待整个进程组
pid, err = unix.Wait4(-pgid, &wstatus, unix.WALL, &rusage)
```

### 3. 安全考虑
```
exec前：
- 使用PID等待，避免跟踪到无关进程
- 确保进程正确启动和设置

exec后：
- 使用PGID等待，跟踪所有相关进程
- 进程组关系已确定，安全可控
```

### 4. 状态变化处理机制

```
1. 信号通知：
   - 子进程状态变化时，内核发送 SIGCHLD 信号给父进程
   - 信号会被排队，确保不会丢失

2. 状态保持：
   - 进程退出后变成僵尸进程
   - 状态信息保持到父进程通过 wait4 获取
   - 资源使用统计会被保存

3. 多进程处理：
   for {
       pid, err = Wait4(-pgid, &wstatus, WALL, &rusage)
       if pid > 0 {
           // 处理一个子进程的状态变化
           handleStateChange(pid, wstatus)
           // 循环继续，处理下一个状态变化
       }
   }

4. 并发安全：
   - 内核保证状态变化的原子性
   - 信号排队机制防止信息丢失
   - 即使进程快速创建和销毁也能正确跟踪
```

### 5. 实际应用示例

```go
// 1. 处理频繁创建/销毁的场景
func handleFrequentProcesses() {
    for {
        pid, err := unix.Wait4(-pgid, &wstatus, unix.WALL, &rusage)
        if err != nil {
            break
        }
        
        switch {
        case WIFEXITED(wstatus):
            // 正常退出
            handleExit(pid, wstatus.ExitStatus())
            
        case WIFSIGNALED(wstatus):
            // 被信号终止
            handleSignal(pid, wstatus.Signal())
            
        case WIFSTOPPED(wstatus):
            // 被停止
            handleStop(pid, wstatus.StopSignal())
        }
        
        // 继续循环处理其他状态变化
    }
}

// 2. 资源统计收集
type ProcessStats struct {
    ExitCode    int
    UserTime    time.Duration
    SystemTime  time.Duration
    MaxRSS      int64
    // ...
}

func collectStats(rusage *unix.Rusage) ProcessStats {
    return ProcessStats{
        UserTime:   time.Duration(rusage.Utime.Nano()),
        SystemTime: time.Duration(rusage.Stime.Nano()),
        MaxRSS:     rusage.Maxrss,
    }
}
```

## 最佳实践

1. 进程创建和跟踪：
```go
// 创建进程时设置新组
if pid == 0 {  // 子进程
    setpgid(0, 0)
    execve(...)
}

// 父进程中跟踪
if pid > 0 {  // 父进程
    // 先等待特定进程
    Wait4(pid, ...)
    
    // exec后等待整个组
    Wait4(-pid, ...)
}
```

2. 资源管理：
```go
// 可以针对整个进程组统一管理
func manageGroup(pgid int) {
    // 设置资源限制
    setrlimit(...)
    
    // 信号控制
    kill(-pgid, signal)
    
    // 清理资源
    cleanup(-pgid)
}
```

这种设计确保了：
1. 进程跟踪的精确性和安全性
2. 资源隔离和管理的有效性
3. 进程组生命周期的可控性
