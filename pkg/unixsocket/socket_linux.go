// Package unixsocket provides wrapper for Linux unix socket to send and recv oob messages
// including fd and user credential.
package unixsocket

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"syscall"
)

// oob size default to page size
// OOB (Out-Of-Band) 数据大小默认为一个内存页大小 4KB
// 这个大小足够存储文件描述符和用户凭证信息
const oobSize = 4 << 10 // 4kb

// Socket 封装了 Unix domain socket 连接
// 包含了发送和接收缓冲区，用于处理 OOB 消息
type Socket struct {
	*net.UnixConn           // 内嵌 Unix socket 连接
	sendBuff    []byte      // OOB 发送缓冲区
	recvBuff    []byte      // OOB 接收缓冲区
}

// Msg 表示 Unix socket 的带外(OOB)消息
// 可以包含文件描述符和用户凭证信息
type Msg struct {
	Fds  []int          // Unix 权限，用于传递文件描述符
	Cred *syscall.Ucred // Unix 用户凭证，包含 UID、GID 等信息
}

// newSocket 创建一个新的 Socket 实例
// 初始化发送和接收缓冲区
func newSocket(conn *net.UnixConn) *Socket {
	return &Socket{
		UnixConn: conn,
		sendBuff: make([]byte, oobSize),
		recvBuff: make([]byte, oobSize),
	}
}

// NewSocket 使用现有的 Unix socket 文件描述符创建 Socket 结构
// 参数:
//   - fd: Unix socket 文件描述符
// 特性:
//   - 设置为非阻塞模式
//   - 设置 close-on-exec 标志，避免文件描述符泄漏
//   - 需要 SOCK_SEQPACKET 类型的 socket 以保证可靠传输
//   - 如果需要传递用户凭证，需要设置 SO_PASSCRED 选项
func NewSocket(fd int) (*Socket, error) {
	// 设置非阻塞模式
	syscall.SetNonblock(fd, true)
	// 设置 close-on-exec 标志
	syscall.CloseOnExec(fd)

	// 将文件描述符转换为 File 对象
	file := os.NewFile(uintptr(fd), "unix-socket")
	if file == nil {
		return nil, fmt.Errorf("NewSocket: %d is not a valid fd", fd)
	}
	defer file.Close()

	// 将 File 对象转换为 net.Conn
	conn, err := net.FileConn(file)
	if err != nil {
		return nil, err
	}

	// 确保是 Unix domain socket 连接
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("NewSocket: %d is not a valid unix socket connection", fd)
	}
	return newSocket(unixConn), nil
}

// NewSocketPair 创建一对相连的 Unix domain socket
// 使用 SOCK_SEQPACKET 类型确保可靠的数据传输
// 返回两个 Socket 对象，可用于进程间通信
func NewSocketPair() (*Socket, *Socket, error) {
	// 创建 socket 对，使用 SOCK_SEQPACKET 类型和 SOCK_CLOEXEC 标志
	fd, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("NewSocketPair: failed to call socketpair %v", err)
	}

	// 创建发送端 socket
	ins, err := NewSocket(fd[0])
	if err != nil {
		syscall.Close(fd[0])
		syscall.Close(fd[1])
		return nil, nil, fmt.Errorf("NewSocketPair: failed to call NewSocket on sender %v", err)
	}

	// 创建接收端 socket
	outs, err := NewSocket(fd[1])
	if err != nil {
		ins.Close()
		syscall.Close(fd[1])
		return nil, nil, fmt.Errorf("NewSocketPair: failed to call NewSocket receiver %v", err)
	}

	return ins, outs, nil
}

// SetPassCred 设置 socket 的 SO_PASSCRED 选项
// 启用后可以在消息中传递用户凭证信息
func (s *Socket) SetPassCred(option int) error {
	sysconn, err := s.SyscallConn()
	if err != nil {
		return err
	}
	return sysconn.Control(func(fd uintptr) {
		syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, option)
	})
}

// SendMsg 发送消息和相关的 OOB 数据
// 参数:
//   - b: 要发送的数据
//   - m: OOB 消息，可包含文件描述符和用户凭证
func (s *Socket) SendMsg(b []byte, m Msg) error {
	// 准备 OOB 数据缓冲区
	oob := bytes.NewBuffer(s.sendBuff[:0])
	if len(m.Fds) > 0 {
		// 编码文件描述符
		oob.Write(syscall.UnixRights(m.Fds...))
	}
	if m.Cred != nil {
		// 编码用户凭证
		oob.Write(syscall.UnixCredentials(m.Cred))
	}

	// 发送消息和 OOB 数据
	_, _, err := s.WriteMsgUnix(b, oob.Bytes(), nil)
	if err != nil {
		return err
	}
	return nil
}

// RecvMsg 接收消息和解析 OOB 数据
// 参数:
//   - b: 接收数据的缓冲区
// 返回:
//   - 接收的字节数
//   - 解析后的 OOB 消息
//   - 可能的错误
func (s *Socket) RecvMsg(b []byte) (int, Msg, error) {
	var msg Msg
	// 接收消息和 OOB 数据
	n, oobn, _, _, err := s.ReadMsgUnix(b, s.recvBuff)
	if err != nil {
		return 0, msg, err
	}
	// 解析 OOB 消息
	msgs, err := syscall.ParseSocketControlMessage(s.recvBuff[:oobn])
	if err != nil {
		return 0, msg, err
	}
	msg, err = parseMsg(msgs)
	if err != nil {
		return 0, msg, err
	}
	return n, msg, nil
}

// parseMsg 解析 socket 控制消息
// 支持两种类型的 OOB 数据:
//   - SCM_CREDENTIALS: 用户凭证
//   - SCM_RIGHTS: 文件描述符
func parseMsg(msgs []syscall.SocketControlMessage) (msg Msg, err error) {
	// 确保在发生错误时关闭所有打开的文件描述符
	defer func() {
		if err != nil {
			for _, f := range msg.Fds {
				syscall.Close(f)
			}
			msg.Fds = nil
		}
	}()
	
	// 遍历所有控制消息
	for _, m := range msgs {
		if m.Header.Level != syscall.SOL_SOCKET {
			continue
		}

		switch m.Header.Type {
		case syscall.SCM_CREDENTIALS:
			// 解析用户凭证信息
			cred, err := syscall.ParseUnixCredentials(&m)
			if err != nil {
				return msg, err
			}
			msg.Cred = cred

		case syscall.SCM_RIGHTS:
			// 解析文件描述符
			fds, err := syscall.ParseUnixRights(&m)
			if err != nil {
				return msg, err
			}
			msg.Fds = fds
		}
	}
	return msg, nil
}
