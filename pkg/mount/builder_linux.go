package mount

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	// bind 定义了绑定挂载的默认标志位组合：
	// - MS_BIND: 创建绑定挂载
	// - MS_NOSUID: 禁用 SUID 和 SGID 位
	// - MS_PRIVATE: 确保挂载点是私有的，不会传播到其他命名空间
	// - MS_REC: 递归应用到所有子挂载点
	bind = unix.MS_BIND | unix.MS_NOSUID | unix.MS_PRIVATE | unix.MS_REC

	// mFlag 定义了通用挂载的默认标志位组合：
	// - MS_NOSUID: 禁用 SUID 和 SGID 位
	// - MS_NOATIME: 不更新文件访问时间，提高性能
	// - MS_NODEV: 禁止访问设备文件
	mFlag = unix.MS_NOSUID | unix.MS_NOATIME | unix.MS_NODEV
)

// NewDefaultBuilder 创建一个默认的构建器，预配置了最小根文件系统所需的基本挂载点：
// - /usr: 系统程序和库文件
// - /lib 和 /lib64: 系统库文件
// - /bin: 基本命令
// 所有挂载点默认以只读方式挂载
func NewDefaultBuilder() *Builder {
	return NewBuilder().
		WithBind("/usr", "usr", true).
		WithBind("/lib", "lib", true).
		WithBind("/lib64", "lib64", true).
		WithBind("/bin", "bin", true)
}

// Build 根据构建器中的配置创建系统调用参数序列
// 返回值：
// - []SyscallParams: 包含所有挂载操作的系统调用参数
// - error: 如果在准备过程中发生错误则返回
func (b *Builder) Build() ([]SyscallParams, error) {
	var err error
	ret := make([]SyscallParams, 0, len(b.Mounts))
	for _, m := range b.Mounts {
		var mknod bool
		if mknod, err = isBindMountFileOrNotExists(m); err != nil {
			return nil, err
		}
		sp, err := m.ToSyscall()
		if err != nil {
			return nil, err
		}
		sp.MakeNod = mknod
		ret = append(ret, *sp)
	}
	return ret, nil
}

// FilterNotExist 从构建器中移除源路径不存在的绑定挂载
// 这在处理可选的系统目录时很有用，比如某些系统没有 /lib64
// 返回构建器自身以支持链式调用
func (b *Builder) FilterNotExist() *Builder {
	rt := b.Mounts[:0]
	for _, m := range b.Mounts {
		if m.IsBindMount() {
			if _, err := os.Stat(m.Source); os.IsNotExist(err) {
				continue
			}
		}
		rt = append(rt, m)
	}
	b.Mounts = rt
	return b
}

// isBindMountFileOrNotExists 检查绑定挂载的源路径状态
// 返回值：
// - bool: 如果是文件（非目录）的绑定挂载则返回 true
// - error: 如果源路径不存在或检查过程中发生错误则返回
func isBindMountFileOrNotExists(m Mount) (bool, error) {
	if m.IsBindMount() {
		if fi, err := os.Stat(m.Source); os.IsNotExist(err) {
			return false, err
		} else if !fi.IsDir() {
			return true, err
		}
	}
	return false, nil
}

// WithMounts 将多个挂载点添加到构建器中
// 参数：
// - m: 要添加的挂载点列表
// 返回构建器自身以支持链式调用
func (b *Builder) WithMounts(m []Mount) *Builder {
	b.Mounts = append(b.Mounts, m...)
	return b
}

// WithMount 将单个挂载点添加到构建器中
// 参数：
// - m: 要添加的挂载点
// 返回构建器自身以支持链式调用
func (b *Builder) WithMount(m Mount) *Builder {
	b.Mounts = append(b.Mounts, m)
	return b
}

// WithBind 添加一个绑定挂载到构建器中
// 参数：
// - source: 源路径（宿主机上的路径）
// - target: 目标路径（容器内的路径）
// - readonly: 是否以只读方式挂载
// 返回构建器自身以支持链式调用
func (b *Builder) WithBind(source, target string, readonly bool) *Builder {
	var flags uintptr = bind
	if readonly {
		flags |= unix.MS_RDONLY
	}
	b.Mounts = append(b.Mounts, Mount{
		Source: source,
		Target: target,
		Flags:  flags,
	})
	return b
}

// WithTmpfs 添加一个 tmpfs 临时文件系统挂载到构建器中
// 参数：
// - target: 挂载点路径
// - data: 挂载选项（如 "size=64m,mode=755"）
// 返回构建器自身以支持链式调用
func (b *Builder) WithTmpfs(target, data string) *Builder {
	b.Mounts = append(b.Mounts, Mount{
		Source: "tmpfs",
		Target: target,
		FsType: "tmpfs",
		Flags:  mFlag,
		Data:   data,
	})
	return b
}

// WithProc 添加一个只读的 proc 文件系统挂载
// 这是 WithProcRW(false) 的快捷方式
// 返回构建器自身以支持链式调用
func (b *Builder) WithProc() *Builder {
	return b.WithProcRW(false)
}

// WithProcRW 添加 proc 文件系统挂载，可以指定是否为只读
// 参数：
// - canWrite: 如果为 true，则允许写入操作
// 返回构建器自身以支持链式调用
func (b *Builder) WithProcRW(canWrite bool) *Builder {
	var flags uintptr = unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC
	if !canWrite {
		flags |= unix.MS_RDONLY
	}
	b.Mounts = append(b.Mounts, Mount{
		Source: "proc",
		Target: "proc",
		FsType: "proc",
		Flags:  flags,
	})
	return b
}

// String 实现 Stringer 接口，返回构建器中所有挂载点的字符串表示
// 主要用于调试和日志输出
func (b Builder) String() string {
	var sb strings.Builder
	sb.WriteString("Mounts: ")
	for i, m := range b.Mounts {
		sb.WriteString(m.String())
		if i != len(b.Mounts)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String()
}
