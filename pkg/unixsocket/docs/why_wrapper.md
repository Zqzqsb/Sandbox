# 为什么要封装 Unix Socket？

## 背景

在 GoJudgeSandbox 项目中，我们需要在沙箱环境中安全地执行用户代码。这就需要进程间通信（IPC）来：
1. 在沙箱内外传递数据
2. 控制沙箱进程的生命周期
3. 传递文件描述符和用户凭证

## 封装的原因

### 1. 简化复杂的系统调用

原生的 Unix socket 系统调用使用起来较为复杂：

```go
// 原生系统调用
fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
if err != nil {
    return err
}
cred := &syscall.Ucred{/*...*/}
oob := syscall.UnixCredentials(cred)
_, _, err = syscall.Sendmsg(fd, data, oob, nil, 0)

// 封装后
socket.SendMsg(data, Msg{Cred: cred})
```

### 2. 提供可靠的资源管理

1. **自动化的资源清理**
   ```go
   // 封装确保资源正确释放
   socket, _ := NewSocket(fd)
   defer socket.Close()
   ```

2. **文件描述符管理**
   ```go
   // 自动处理文件描述符的传递和清理
   msg, _ := socket.RecvMsg(buf)
   defer func() {
       for _, fd := range msg.Fds {
           syscall.Close(fd)
       }
   }()
   ```

### 3. 增强的错误处理

1. **统一的错误处理机制**
   ```go
   if err := socket.SendMsg(data, msg); err != nil {
       // 统一的错误处理逻辑
   }
   ```

2. **更好的错误上下文**
   ```go
   // 提供更有意义的错误信息
   return fmt.Errorf("NewSocket: %d is not a valid unix socket connection", fd)
   ```

### 4. 安全性保证

1. **强制设置安全选项**
   ```go
   // 自动设置 close-on-exec 标志
   syscall.CloseOnExec(fd)
   ```

2. **权限控制**
   ```go
   // 凭证传递的安全封装
   socket.SetPassCred(1)
   ```

### 5. 性能优化

1. **缓冲区管理**
   ```go
   // 预分配固定大小的缓冲区
   const oobSize = 4 << 10
   type Socket struct {
       sendBuff []byte
       recvBuff []byte
   }
   ```

2. **避免重复分配**
   ```go
   // 重用缓冲区
   oob := bytes.NewBuffer(s.sendBuff[:0])
   ```

## 实际应用场景

### 1. 沙箱进程控制

```go
// 在沙箱内外建立可靠的通信通道
sender, receiver, _ := NewSocketPair()
cmd := exec.Command("sandbox-process")
cmd.ExtraFiles = append(cmd.ExtraFiles, receiver.File())
```

### 2. 文件描述符传递

```go
// 安全地传递文件描述符
socket.SendMsg(data, Msg{
    Fds: []int{logFile.Fd(), inputFile.Fd()},
})
```

### 3. 用户权限验证

```go
// 验证连接的进程身份
_, msg, _ := socket.RecvMsg(buf)
if msg.Cred != nil {
    uid := msg.Cred.Uid
    // 进行权限检查
}
```

## 优势总结

1. **易用性**
   - 简化的 API 接口
   - 统一的错误处理
   - 自动的资源管理

2. **可靠性**
   - 资源泄漏防护
   - 健壮的错误处理
   - 自动化的清理机制

3. **安全性**
   - 强制安全选项
   - 权限控制
   - 凭证验证

4. **性能**
   - 优化的缓冲区管理
   - 减少内存分配
   - 高效的资源利用

## 使用建议

1. **初始化**
   ```go
   // 优先使用 NewSocketPair 创建配对的 socket
   sender, receiver, err := NewSocketPair()
   ```

2. **错误处理**
   ```go
   // 始终检查错误并正确清理资源
   if err != nil {
       socket.Close()
       return err
   }
   ```

3. **资源管理**
   ```go
   // 使用 defer 确保资源释放
   defer socket.Close()
   ```

## 参考

1. Go 标准库
   - `syscall`
   - `net`
   - `os`

2. Linux 系统调用
   - `socket(2)`
   - `sendmsg(2)`
   - `recvmsg(2)`
