package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Mount 执行挂载系统调用
// 如果是只读绑定挂载，需要重新挂载一次来确保只读属性生效
func (m *Mount) Mount() error {
	// 确保挂载目标存在（目录或文件）
	if err := ensureMountTargetExists(m.Source, m.Target); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// 执行挂载系统调用
	if err := syscall.Mount(m.Source, m.Target, m.FsType, m.Flags, m.Data); err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	// 对于只读绑定挂载，需要重新挂载一次
	// 因为在第一次挂载时 MS_RDONLY 标志会被忽略
	const bindRo = syscall.MS_BIND | syscall.MS_RDONLY
	if m.Flags&bindRo == bindRo {
		if err := syscall.Mount("", m.Target, m.FsType, m.Flags|syscall.MS_REMOUNT, m.Data); err != nil {
			return fmt.Errorf("remount: %w", err)
		}
	}
	return nil
}

// IsBindMount 判断是否为绑定挂载
// 通过检查 MS_BIND 标志位来确定
func (m Mount) IsBindMount() bool {
	return m.Flags&syscall.MS_BIND == syscall.MS_BIND
}

// IsReadOnly 判断是否为只读挂载
// 通过检查 MS_RDONLY 标志位来确定
func (m Mount) IsReadOnly() bool {
	return m.Flags&syscall.MS_RDONLY == syscall.MS_RDONLY
}

// IsTmpFs 判断是否为 tmpfs 文件系统
func (m Mount) IsTmpFs() bool {
	return m.FsType == "tmpfs"
}

// ensureMountTargetExists 确保挂载目标存在
// 如果源是文件，则创建目标文件
// 如果源是目录，则创建目标目录
func ensureMountTargetExists(source, target string) error {
	// 判断源是文件还是目录
	isFile := false
	if fi, err := os.Stat(source); err == nil {
		isFile = !fi.IsDir()
	}
	// 获取需要创建的目录路径
	dir := target
	if isFile {
		dir = filepath.Dir(target)
	}
	// 递归创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// 如果源是文件，则创建目标文件
	if isFile {
		if err := syscall.Mknod(target, 0755, 0); err != nil {
			// 双重检查文件是否已存在
			// 避免并发创建的问题
			f, err1 := os.Lstat(target)
			if err1 == nil && f.Mode().IsRegular() {
				return nil
			}
			return err
		}
	}
	return nil
}

// String 返回挂载点的字符串表示
// 包含挂载类型、源路径、目标路径和读写权限等信息
func (m Mount) String() string {
	// 确定读写权限标志
	flag := "rw"
	if m.Flags&syscall.MS_RDONLY == syscall.MS_RDONLY {
		flag = "ro"
	}
	// 根据不同的挂载类型返回不同格式的字符串
	switch {
	case m.Flags&syscall.MS_BIND == syscall.MS_BIND:
		return fmt.Sprintf("bind[%s:%s:%s]", m.Source, m.Target, flag)

	case m.FsType == "tmpfs":
		return fmt.Sprintf("tmpfs[%s]", m.Target)

	case m.FsType == "proc":
		return fmt.Sprintf("proc[%s]", flag)

	default:
		return fmt.Sprintf("mount[%s,%s:%s:%x,%s]", m.FsType, m.Source, m.Target, m.Flags, m.Data)
	}
}
