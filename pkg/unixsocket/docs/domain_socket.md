# Unix Domain Socket

## 什么是 Domain Socket？

Domain Socket（也称为 IPC Socket 或 Unix Domain Socket）是一种进程间通信（IPC）机制，它使用文件系统作为寻址方式的通信协议。与网络 socket 不同，Domain Socket 仅用于同一台计算机上的进程间通信。

### 特点

1. **高性能**
   - 不需要经过网络协议栈
   - 无需打包/解包 TCP/IP 协议
   - 零拷贝数据传输
   - 比 TCP/IP socket 快约 50%

2. **基于文件系统**
   - 使用文件路径作为通信地址
   - 支持标准的文件系统权限控制
   - Socket 文件通常位于 `/tmp` 或 `/var/run` 目录

3. **可靠性**
   - 保证数据传输的顺序
   - 不会丢失或重复数据
   - 支持流式和数据报两种模式

### 通信模式

1. **SOCK_STREAM（流式）**
   ```go
   // 创建流式 socket
   syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
   ```
   - 类似 TCP，提供可靠的双向通信
   - 数据无边界，需要自行处理消息分割
   - 适合大量数据传输

2. **SOCK_DGRAM（数据报）**
   ```go
   // 创建数据报 socket
   syscall.Socket(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
   ```
   - 类似 UDP，保留消息边界
   - 每次读写是原子的
   - 适合小数据包传输

3. **SOCK_SEQPACKET（序列包）**
   ```go
   // 创建序列包 socket
   syscall.Socket(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
   ```
   - 结合了 STREAM 和 DGRAM 的特点
   - 可靠的面向消息的双向通信
   - 保留消息边界

### 使用场景

1. **系统服务通信**
   - systemd 与服务之间的通信
   - Docker daemon 与客户端通信
   - X Window System 的客户端服务器通信

2. **数据库连接**
   - MySQL 本地连接
   - PostgreSQL 本地连接
   - Redis Unix Socket 连接

3. **Web 服务器**
   - Nginx 与 PHP-FPM 通信
   - Unix Socket 反向代理
   - 应用服务器与 Web 服务器通信

### 代码示例

1. **服务器端**
```go
func server() error {
    // 删除可能存在的旧 socket 文件
    os.Remove("/tmp/example.sock")
    
    // 创建并监听 Unix domain socket
    listener, err := net.Listen("unix", "/tmp/example.sock")
    if err != nil {
        return err
    }
    defer listener.Close()
    
    // 接受连接
    conn, err := listener.Accept()
    if err != nil {
        return err
    }
    defer conn.Close()
    
    // 处理连接
    buf := make([]byte, 1024)
    n, err := conn.Read(buf)
    if err != nil {
        return err
    }
    
    fmt.Printf("Received: %s\n", buf[:n])
    return nil
}
```

2. **客户端**
```go
func client() error {
    // 连接到 Unix domain socket
    conn, err := net.Dial("unix", "/tmp/example.sock")
    if err != nil {
        return err
    }
    defer conn.Close()
    
    // 发送数据
    _, err = conn.Write([]byte("Hello, Unix domain socket!"))
    return err
}
```

### 权限和安全

1. **文件系统权限**
   - Socket 文件遵循标准的 Unix 权限模型
   - 可以使用 chmod/chown 设置访问权限
   - 支持 SELinux 和 AppArmor 等安全机制

2. **凭证传递**
   - 支持通过 SCM_CREDENTIALS 传递进程凭证
   - 可以验证连接双方的身份
   - 支持细粒度的访问控制

3. **安全注意事项**
   - 正确设置 socket 文件权限
   - 避免在全局可写目录中创建 socket
   - 及时清理未使用的 socket 文件

### 性能优化

1. **缓冲区调优**
   ```go
   syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 65536)
   syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 65536)
   ```

2. **非阻塞模式**
   ```go
   syscall.SetNonblock(fd, true)
   ```

3. **超时设置**
   ```go
   conn.SetDeadline(time.Now().Add(timeout))
   ```

### 调试工具

1. **ss 命令**
   ```bash
   ss -x  # 显示所有 Unix domain socket
   ```

2. **netstat 命令**
   ```bash
   netstat -ax  # 显示 Unix domain socket 统计
   ```

3. **lsof 命令**
   ```bash
   lsof -U  # 列出使用 Unix domain socket 的进程
   ```

### 参考资料

1. man pages:
   - unix(7)
   - socket(7)
   - socket(2)
   - bind(2)

2. 标准:
   - POSIX.1-2008
   - Single UNIX Specification, Version 4

3. 相关文档:
   - Linux Network Programming
   - Advanced Programming in the UNIX Environment
