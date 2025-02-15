# Clone 和 Unshare 的协同工作机制

## 1. 基本概念

### 1.1 Clone 系统调用
```mermaid
graph TD
    A[Clone 系统调用] --> B[创建新进程]
    B --> C[设置命名空间标志]
    B --> D[建立父子关系]
    
    C --> E[CLONE_NEWUSER]
    C --> F[CLONE_NEWPID]
    C --> G[其他命名空间]
```

### 1.2 Unshare 机制
```mermaid
graph TD
    A[Unshare 机制] --> B[分离命名空间]
    B --> C[用户命名空间]
    B --> D[挂载命名空间]
    B --> E[其他资源]
```

## 2. 协同工作流程

### 2.1 基本流程
```mermaid
sequenceDiagram
    participant P as 父进程
    participant C as 子进程
    participant NS as 命名空间
    
    P->>P: 准备 Clone 标志
    P->>C: Clone 系统调用
    Note over C: 新进程创建
    C->>NS: 检查 Unshare 需求
    C->>P: 等待父进程配置
    P->>NS: 设置 UID/GID 映射
    P->>C: 通知配置完成
    C->>C: 继续执行
```

### 2.2 命名空间处理顺序
1. **用户命名空间**：
   ```go
   // 检查是否需要用户命名空间
   unshareUser = r.CloneFlags&unix.CLONE_NEWUSER == unix.CLONE_NEWUSER
   ```

2. **其他命名空间**：
   ```go
   // UnshareFlags 定义
   UnshareFlags = unix.CLONE_NEWIPC | unix.CLONE_NEWNET | 
                 unix.CLONE_NEWNS | unix.CLONE_NEWPID | 
                 unix.CLONE_NEWUSER | unix.CLONE_NEWUTS | 
                 unix.CLONE_NEWCGROUP
   ```

## 3. 同步机制

### 3.1 父子进程同步
```mermaid
sequenceDiagram
    participant P as 父进程
    participant Pipe as 管道
    participant C as 子进程
    
    C->>Pipe: 关闭写入端
    C->>Pipe: 等待读取
    P->>P: 设置 UID/GID 映射
    P->>Pipe: 写入结果
    C->>C: 收到结果继续
```

### 3.2 错误处理
```go
if unshareUser {
    // 1. 读取父进程的配置结果
    r1, _, err1 = syscall.RawSyscall(syscall.SYS_READ, 
        uintptr(pipe), 
        uintptr(unsafe.Pointer(&err2)), 
        unsafe.Sizeof(err2))
        
    // 2. 检查读取是否成功
    if err1 != 0 {
        childExitError(pipe, LocUnshareUserRead, err1)
    }
    
    // 3. 验证数据完整性
    if r1 != unsafe.Sizeof(err2) {
        err1 = syscall.EINVAL
        childExitError(pipe, LocUnshareUserRead, err1)
    }
}
```

## 4. 权限和安全

### 4.1 权限控制流程
```mermaid
graph TD
    A[进程创建] --> B{需要用户命名空间?}
    B -- 是 --> C[等待权限映射]
    B -- 否 --> D[继续执行]
    C --> E[验证映射]
    E --> F[设置其他命名空间]
```

### 4.2 安全考虑
1. **用户命名空间优先**：
   - 必须首先建立用户命名空间
   - 确保后续操作有正确权限

2. **权限映射**：
   - 在父进程中设置
   - 子进程等待完成
   - 确保安全性

## 5. 实际应用场景

### 5.1 沙箱环境
```mermaid
graph TD
    A[沙箱创建] --> B[Clone 新进程]
    B --> C[Unshare 资源]
    C --> D[设置限制]
    D --> E[执行目标程序]
```

### 5.2 资源隔离
1. **文件系统**：
   - 通过 CLONE_NEWNS
   - 配合 pivot_root

2. **网络**：
   - 通过 CLONE_NEWNET
   - 配置独立网络栈

3. **进程空间**：
   - 通过 CLONE_NEWPID
   - 独立的进程树

### 总结

```mermaid
graph TD
    A[Clone 动作] --> B[创建进程]
    B --> C[指定 Unshare 标志]
    C --> D[Unshare 机制生效]
    D --> E[持续性隔离]
    
    style A fill:#f9f,stroke:#333
    style D fill:#bbf,stroke:#333
```