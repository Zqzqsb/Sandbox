// Package forkexec 提供进程创建和执行的功能
package forkexec

import (
	"golang.org/x/sys/unix"
)

// 定义 syscall 包中缺少的常量
const (
	// SECCOMP_SET_MODE_STRICT 是 seccomp 的严格模式
	// 在此模式下，只允许 read、write、_exit(2) 和 sigreturn 系统调用
	SECCOMP_SET_MODE_STRICT = 0

	// SECCOMP_SET_MODE_FILTER 是 seccomp 的过滤器模式
	// 允许使用 BPF 过滤器定义允许的系统调用
	SECCOMP_SET_MODE_FILTER = 1

	// SECCOMP_FILTER_FLAG_TSYNC 表示同步所有线程的 seccomp 过滤器
	// 确保所有线程都使用相同的系统调用过滤规则
	SECCOMP_FILTER_FLAG_TSYNC = 1

	// UnshareFlags 定义了创建新命名空间的标志位组合
	// CLONE_NEWIPC: 新的 IPC 命名空间
	// CLONE_NEWNET: 新的网络命名空间
	// CLONE_NEWNS: 新的挂载命名空间
	// CLONE_NEWPID: 新的 PID 命名空间
	// CLONE_NEWUSER: 新的用户命名空间
	// CLONE_NEWUTS: 新的 UTS（主机名和域名）命名空间
	// CLONE_NEWCGROUP: 新的 cgroup 命名空间
	UnshareFlags = unix.CLONE_NEWIPC | unix.CLONE_NEWNET | unix.CLONE_NEWNS |
		unix.CLONE_NEWPID | unix.CLONE_NEWUSER | unix.CLONE_NEWUTS | unix.CLONE_NEWCGROUP

	// bindRo 定义了只读绑定挂载的标志位组合
	// MS_BIND: 创建绑定挂载
	// MS_RDONLY: 设置为只读
	bindRo = unix.MS_BIND | unix.MS_RDONLY
)

// 用于 unshare 重新挂载和 pivot_root 操作的常量字节数组
var (
	// none 用于 mount 系统调用，表示无文件系统类型
	none = []byte("none\000")
	// slash 表示根目录路径
	slash = []byte("/\000")
	// empty 表示空字符串
	empty = []byte("\000")
	// tmpfs 表示临时文件系统类型
	tmpfs = []byte("tmpfs\000")

	// oldRoot 是 pivot_root 操作时用于存放旧根目录的目录名
	oldRoot = []byte("old_root\000")

	// setGIDAllow 和 setGIDDeny 用于配置用户命名空间的 GID 映射策略
	setGIDAllow = []byte("allow")
	setGIDDeny  = []byte("deny")

	// _AT_FDCWD 是一个特殊的文件描述符值，表示当前工作目录
	// Go 不允许 uintptr 类型的常量为负数，所以使用变量
	_AT_FDCWD = unix.AT_FDCWD

	// dropCapHeader 用于删除所有能力（capabilities）的头部结构
	dropCapHeader = unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0,
	}

	// dropCapData 用于删除所有能力的数据结构
	// 将所有能力位都设置为 0
	dropCapData = unix.CapUserData{
		Effective:   0, // 有效能力集
		Permitted:   0, // 允许能力集
		Inheritable: 0, // 可继承能力集
	}

	// etxtbsyRetryInterval 定义了遇到 ETXTBSY 错误时的重试间隔
	// 设置为 1 毫秒 (1 * 1000 * 1000 纳秒)
	etxtbsyRetryInterval = unix.Timespec{
		Nsec: 1 * 1000 * 1000,
	}
)

// Linux 安全位（Secure Bits）的常量定义
const (
	// _SECURE_NOROOT: 禁止 root 用户的特权
	_SECURE_NOROOT = 1 << iota
	// _SECURE_NOROOT_LOCKED: 锁定 NOROOT 设置，防止被修改
	_SECURE_NOROOT_LOCKED

	// _SECURE_NO_SETUID_FIXUP: 禁用 setuid 程序的特权提升
	_SECURE_NO_SETUID_FIXUP
	// _SECURE_NO_SETUID_FIXUP_LOCKED: 锁定 NO_SETUID_FIXUP 设置
	_SECURE_NO_SETUID_FIXUP_LOCKED

	// _SECURE_KEEP_CAPS: 在 uid 变更时保留能力
	_SECURE_KEEP_CAPS
	// _SECURE_KEEP_CAPS_LOCKED: 锁定 KEEP_CAPS 设置
	_SECURE_KEEP_CAPS_LOCKED

	// _SECURE_NO_CAP_AMBIENT_RAISE: 禁止提升 ambient 能力集
	_SECURE_NO_CAP_AMBIENT_RAISE
	// _SECURE_NO_CAP_AMBIENT_RAISE_LOCKED: 锁定 NO_CAP_AMBIENT_RAISE 设置
	_SECURE_NO_CAP_AMBIENT_RAISE_LOCKED
)
