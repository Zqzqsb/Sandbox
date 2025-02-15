package forkexec

import (
	"syscall"

	"github.com/zqzqsb/sandbox/pkg/mount"
	"github.com/zqzqsb/sandbox/pkg/rlimit"
)

// Runner 是一个配置结构体，包含了执行路径、参数以及资源限制等配置
// 它可以创建用于 ptrace 跟踪的被跟踪进程
// 同时也可以在新的命名空间中创建隔离的进程
type Runner struct {
	// Args 和 Env 用于子进程的 execve 系统调用
	// Args: 命令行参数数组，Args[0] 是要执行的程序路径
	// Env: 环境变量数组，格式为 "KEY=VALUE"
	Args []string
	Env  []string

	// ExecFile 如果定义了，将使用 fd_execve 系统调用
	// 这允许通过文件描述符而不是路径名来执行程序
	ExecFile uintptr

	// RLimits 定义了进程的资源限制
	// 通过 setrlimit 系统调用设置，如 CPU 时间、内存等限制
	RLimits []rlimit.RLimit

	// Files 定义了新进程的文件描述符映射
	// 索引从 0 开始，通常 0,1,2 分别对应 stdin, stdout, stderr
	Files []uintptr

	// WorkDir 设置子进程的工作目录
	// 通过 chdir(dir) 实现
	// 如果设置了 PivotRoot，这个操作会在切换根目录后执行
	WorkDir string

	// Seccomp 定义了系统调用过滤器
	// 用于限制进程可以使用的系统调用
	Seccomp *syscall.SockFprog

	// CloneFlags 定义了创建 Linux 命名空间的标志
	// 在克隆子进程时生效
	// 使用 unshare 系统调用不会加入新的 PID 组
	CloneFlags uintptr

	// Mounts 定义了在 unshare 挂载命名空间后要执行的挂载操作
	// 需要在命名空间内有 CAP_SYS_ADMIN 权限（例如通过 unshare 用户命名空间）
	// 如果提供了 PivotRoot，建议使用相对路径作为目标
	// PivotRoot 会在任何挂载操作前将根目录挂载为 tmpfs
	Mounts []mount.SyscallParams

	// PivotRoot 定义了一个只读的新根目录
	// 必须是绝对路径的目录，通常与 Mounts 一起使用
	// 执行步骤：
	// 1. mount("tmpfs", root, "tmpfs", 0, nil)
	// 2. chdir(root)
	// 3. [执行挂载操作]
	// 4. mkdir("old_root")
	// 5. pivot_root(root, "old_root")
	// 6. umount("old_root", MNT_DETACH)
	// 7. rmdir("old_root")
	// 8. mount("tmpfs", "/", "tmpfs", MS_BIND | MS_REMOUNT | MS_RDONLY | MS_NOATIME | MS_NOSUID, nil)
	PivotRoot string

	// HostName 和 DomainName 在 unshare UTS 和用户命名空间后设置
	// 需要 CAP_SYS_ADMIN 权限
	HostName, DomainName string

	// UIDMappings 和 GIDMappings 用于用户命名空间的 UID/GID 映射
	// 如果映射为空则不进行操作
	UIDMappings []syscall.SysProcIDMap
	GIDMappings []syscall.SysProcIDMap

	// Credential 保存了子进程要使用的用户和组身份信息
	Credential *syscall.Credential

	// SyncFunc 用于父子进程通过套接字对同步状态
	// 会传入子进程的 PID 作为参数
	// 如果 SyncFunc 返回错误，父进程会通知子进程停止并报告错误
	// SyncFunc 在 execve 之前调用，因此可以更准确地跟踪 CPU 使用
	SyncFunc func(int) error

	// Ptrace 控制子进程调用 ptrace(PTRACE_TRACEME)
	// 跟踪器需要调用 runtime.LockOSThread 来使用 ptrace 系统调用
	Ptrace bool

	// NoNewPrivs 通过 prctl(PR_SET_NO_NEW_PRIVS) 禁用对 setuid 进程的调用
	// 当提供 seccomp 过滤器时自动启用
	NoNewPrivs bool

	// StopBeforeSeccomp 在 seccomp 调用前通过 kill(getpid(), SIGSTOP) 等待跟踪器继续
	// 当同时启用 seccomp 过滤器和 ptrace 时自动启用
	// 因为 kill 可能在 seccomp 后不可用，且 execve 可能被 ptrace 跟踪
	// 不能在 seccomp 后停止，因为 kill 可能被 seccomp 过滤器禁用
	StopBeforeSeccomp bool

	// GIDMappingsEnableSetgroups 允许/禁止 setgroups 系统调用
	// 如果 GIDMappings 为 nil 则拒绝
	GIDMappingsEnableSetgroups bool

	// DropCaps 在 execve 前通过 cap_set(self, 0) 删除所有权能
	// 包括有效、允许和可继承的权能集
	// 应该避免设置环境权能
	DropCaps bool

	// UnshareCgroupAfterSync 指定是否在同步后取消共享 cgroup 命名空间
	// （syncFunc 可能会将子进程添加到 cgroup 中）
	UnshareCgroupAfterSync bool

	// CTTY 指定是否将文件描述符 0 设置为控制终端
	CTTY bool
}
