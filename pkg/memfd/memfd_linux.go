package memfd

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// 创建 memfd 的标志位组合：
// MFD_CLOEXEC: 在执行 exec 时自动关闭文件描述符
// MFD_ALLOW_SEALING: 允许对文件进行密封操作
const createFlag = unix.MFD_CLOEXEC | unix.MFD_ALLOW_SEALING

// 只读密封标志位组合：
// F_SEAL_SEAL: 防止进一步添加新的密封
// F_SEAL_SHRINK: 防止文件缩小
// F_SEAL_GROW: 防止文件增长
// F_SEAL_WRITE: 防止写入操作
const roSeal = unix.F_SEAL_SEAL | unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE

// New 创建一个新的 memfd（内存文件）
// 参数：
//   - name: 文件名（仅用于调试目的）
// 返回：
//   - *os.File: 文件对象
//   - error: 错误信息
// 注意：调用者需要负责关闭返回的文件
func New(name string) (*os.File, error) {
	// 调用系统调用创建 memfd
	fd, err := unix.MemfdCreate(name, createFlag)
	if err != nil {
		return nil, fmt.Errorf("memfd: memfd_create failed %v", err)
	}
	// 将文件描述符转换为 Go 的文件对象
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		unix.Close(fd)
		return nil, fmt.Errorf("memfd: NewFile failed for %v", name)
	}
	return file, nil
}

// DupToMemfd 将 reader 中的内容读取到一个只读的 memfd 中
// 这个函数主要用于创建一个内存中的只读文件副本
// 参数：
//   - name: 文件名（仅用于调试）
//   - reader: 数据来源
// 返回：
//   - *os.File: 密封的只读文件
//   - error: 错误信息
func DupToMemfd(name string, reader io.Reader) (*os.File, error) {
	// 创建新的 memfd
	file, err := New(name)
	if err != nil {
		return nil, fmt.Errorf("DupToMemfd: %v", err)
	}
	// 从 reader 复制数据到 memfd
	// 注：如果 reader 是文件，使用 linux syscall sendfile 可能更高效
	if _, err = file.ReadFrom(reader); err != nil {
		file.Close()
		return nil, fmt.Errorf("DupToMemfd: read from %v", err)
	}
	// 将 memfd 设置为只读（添加密封）
	if _, err = unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, roSeal); err != nil {
		file.Close()
		return nil, fmt.Errorf("DupToMemfd: memfd seal %v", err)
	}
	// 将文件指针重置到开始位置
	if _, err := file.Seek(0, 0); err != nil {
		file.Close()
		return nil, fmt.Errorf("DupToMemfd: file seek %v", err)
	}
	return file, nil
}
