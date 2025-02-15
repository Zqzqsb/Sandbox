# Unix Socket OOB (Out-Of-Band) 消息

## 什么是 OOB 消息？

OOB (Out-Of-Band) 消息是 Unix domain socket 提供的一种特殊的带外数据传输机制。与普通的数据传输不同，OOB 消息允许在不影响主数据流的情况下传递一些特殊的控制信息。

在 Unix 系统中，OOB 消息主要用于传递两类特殊数据：

1. 文件描述符 (File Descriptors)
2. 用户凭证 (User Credentials)

### 特点

1. **独立于主数据流**
   - OOB 数据通过单独的控制通道传输
   - 不会与主数据流混合
   - 可以在同一个连接上同时传输普通数据和控制信息

2. **固定大小限制**
   - OOB 数据大小通常限制在一个内存页大小（如 4KB）
   - 这个限制确保了控制消息的处理效率

3. **原子性**
   - OOB 消息的发送和接收是原子操作
   - 保证了控制信息的完整性和一致性

### 主要用途

1. **文件描述符传递**
   ```go
   // 发送文件描述符
   msg := Msg{
       Fds: []int{fd},
   }
   socket.SendMsg(data, msg)
   ```
   - 允许进程间传递打开的文件描述符
   - 实现了文件句柄的共享
   - 常用于进程间通信和权限委派

2. **用户凭证传递**
   ```go
   // 接收用户凭证
   n, msg, err := socket.RecvMsg(buf)
   if msg.Cred != nil {
       uid := msg.Cred.Uid
       gid := msg.Cred.Gid
   }
   ```
   - 传递进程的身份信息（UID、GID）
   - 用于权限验证和访问控制
   - 支持细粒度的安全策略实现

### 实现机制

1. **控制消息格式**
   - 使用 `SocketControlMessage` 结构
   - 包含消息类型和级别信息
   - 支持多种控制消息的组合

2. **系统调用支持**
   ```go
   // 发送带 OOB 的消息
   syscall.SendmsgN(fd, data, oob, nil, 0)
   
   // 接收带 OOB 的消息
   syscall.RecvmsgN(fd, data, oob, 0)
   ```

3. **权限控制**
   - 需要适当的系统权限
   - 可能需要特定的 socket 选项（如 SO_PASSCRED）
   - 受到操作系统安全策略的限制

### 使用注意事项

1. **资源管理**
   - 传递的文件描述符需要正确关闭
   - 避免描述符泄漏
   - 合理管理 OOB 缓冲区大小

2. **错误处理**
   - 处理发送和接收时的错误
   - 确保在错误情况下清理资源
   - 验证接收到的凭证信息

3. **性能考虑**
   - OOB 消息处理有一定开销
   - 避免过度使用
   - 只传递必要的控制信息

### 示例代码

```go
// 创建 socket 对
sender, receiver, err := NewSocketPair()
if err != nil {
    log.Fatal(err)
}
defer sender.Close()
defer receiver.Close()

// 准备要传递的文件描述符
file, err := os.Open("example.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

// 发送文件描述符
msg := Msg{
    Fds: []int{int(file.Fd())},
}
if err := sender.SendMsg([]byte("file"), msg); err != nil {
    log.Fatal(err)
}

// 接收文件描述符
buf := make([]byte, 1024)
n, recvMsg, err := receiver.RecvMsg(buf)
if err != nil {
    log.Fatal(err)
}

// 使用接收到的文件描述符
if len(recvMsg.Fds) > 0 {
    newFile := os.NewFile(uintptr(recvMsg.Fds[0]), "received-file")
    defer newFile.Close()
    // 使用 newFile...
}
```

### 参考资料

1. Unix Network Programming, Volume 1
2. Linux man pages:
   - unix(7)
   - socket(7)
   - cmsg(3)
3. Go 标准库文档:
   - syscall.UnixRights
   - syscall.ParseUnixRights
   - syscall.ParseSocketControlMessage
