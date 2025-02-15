// Package forkexec 实现了 Linux 下的进程创建和执行功能
package forkexec

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// forkAndExecInChild 实现了类似于 src/syscall/exec_linux.go 的功能
// 但增加了更多的安全特性和隔离机制
//
// 参数说明：
// - r: Runner 结构体，包含了进程的配置信息
// - argv0: 要执行的程序路径
// - argv: 程序的参数列表
// - env: 环境变量列表
// - workdir: 工作目录
// - hostname: 主机名
// - domainname: 域名
// - pivotRoot: 新的根目录路径
// - p: 父子进程间通信的管道
//
// 返回值：
// - r1: 子进程的 PID（在父进程中）或 0（在子进程中）
// - err1: 错误码
//
//go:norace
func forkAndExecInChild(r *Runner, argv0 *byte, argv, env []*byte, workdir, hostname, domainname, pivotRoot *byte, p [2]int) (r1 uintptr, err1 syscall.Errno) {
	// 准备文件描述符，避免在 fork 时出现竞态条件
	fd, nextfd := prepareFds(r.Files)

	// 获取 fork 锁，确保在 fork 之前没有其他线程创建新的文件描述符
	// 这些文件描述符可能还没有设置 close-on-exec 标志
	syscall.ForkLock.Lock()

	// 即将调用 fork
	// 从这里开始不能再分配内存或调用非汇编函数
	beforeFork()

	// 通过 clone 系统调用创建新进程
	// UnshareFlags 包含了需要隔离的命名空间标志
	// SIGCHLD 表示子进程结束时向父进程发送信号
	r1, _, err1 = syscall.RawSyscall6(syscall.SYS_CLONE, uintptr(syscall.SIGCHLD)|(r.CloneFlags&UnshareFlags), 0, 0, 0, 0, 0)
	if err1 != 0 || r1 != 0 {
		// 在父进程中，立即返回
		return
	}

	// 以下代码在子进程中执行
	afterForkInChild()
	// 注意：从这里开始不能调用任何 GO 函数

	pipe := p[1]
	var (
		pid         uintptr
		err2        syscall.Errno
		// 检查是否需要创建新的用户命名空间
		unshareUser = r.CloneFlags&unix.CLONE_NEWUSER == unix.CLONE_NEWUSER
	)

	// 关闭管道的写入端
	if _, _, err1 = syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(p[0]), 0, 0); err1 != 0 {
		childExitError(pipe, LocCloseWrite, err1)
	}

	// 如果启用了用户命名空间，需要等待父进程设置 uid/gid 映射
	// 因为在原始命名空间中我们没有权限创建这些映射
	// 同时需要通过管道进行同步
	if unshareUser {
		r1, _, err1 = syscall.RawSyscall(syscall.SYS_READ, uintptr(pipe), uintptr(unsafe.Pointer(&err2)), unsafe.Sizeof(err2))
		if err1 != 0 {
			childExitError(pipe, LocUnshareUserRead, err1)
		}
		if r1 != unsafe.Sizeof(err2) {
			err1 = syscall.EINVAL
			childExitError(pipe, LocUnshareUserRead, err1)
		}
		if err2 != 0 {
			err1 = err2
			childExitError(pipe, LocUnshareUserRead, err1)
		}
	}

	// 获取子进程的 PID
	pid, _, err1 = syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)
	if err1 != 0 {
		childExitError(pipe, LocGetPid, err1)
	}

	// 如果需要设置凭证或者在同步后取消共享 cgroup，
	// 需要保持进程的特权能力，以便后续操作
	if r.Credential != nil || r.UnshareCgroupAfterSync {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_SECUREBITS,
			_SECURE_KEEP_CAPS_LOCKED|_SECURE_NO_SETUID_FIXUP|_SECURE_NO_SETUID_FIXUP_LOCKED, 0)
		if err1 != 0 {
			childExitError(pipe, LocKeepCapability, err1)
		}
	}

	// 为子进程设置凭证信息（来自 exec_linux.go）
	if cred := r.Credential; cred != nil {
		// 设置用户组
		ngroups := uintptr(len(cred.Groups))
		groups := uintptr(0)
		if ngroups > 0 {
			groups = uintptr(unsafe.Pointer(&cred.Groups[0]))
		}
		// 在某些特殊情况下不设置用户组
		if !(r.GIDMappings != nil && !r.GIDMappingsEnableSetgroups && ngroups == 0) && !cred.NoSetGroups {
			_, _, err1 = syscall.RawSyscall(unix.SYS_SETGROUPS, ngroups, groups, 0)
			if err1 != 0 {
				childExitError(pipe, LocSetGroups, err1)
			}
		}
		// 设置组 ID
		_, _, err1 = syscall.RawSyscall(unix.SYS_SETGID, uintptr(cred.Gid), 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocSetGid, err1)
		}
		// 设置用户 ID
		_, _, err1 = syscall.RawSyscall(unix.SYS_SETUID, uintptr(cred.Uid), 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocSetUid, err1)
		}
	}

	// 第一轮文件描述符处理：fd[i] < i => nextfd
	// 避免在重定向时覆盖还未处理的文件描述符
	if pipe < nextfd {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP3, uintptr(pipe), uintptr(nextfd), syscall.O_CLOEXEC)
		if err1 != 0 {
			childExitError(pipe, LocDup3, err1)
		}
		pipe = nextfd
		nextfd++
	}
	// 处理可执行文件的文件描述符
	if r.ExecFile > 0 && int(r.ExecFile) < nextfd {
		// 避免描述符重写
		for nextfd == pipe {
			nextfd++
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP3, r.ExecFile, uintptr(nextfd), syscall.O_CLOEXEC)
		if err1 != 0 {
			childExitError(pipe, LocDup3, err1)
		}
		r.ExecFile = uintptr(nextfd)
		nextfd++
	}
	// 处理其他文件描述符
	for i := 0; i < len(fd); i++ {
		if fd[i] >= 0 && fd[i] < int(i) {
			// 避免描述符重写
			for nextfd == pipe || (r.ExecFile > 0 && nextfd == int(r.ExecFile)) {
				nextfd++
			}
			_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP3, uintptr(fd[i]), uintptr(nextfd), syscall.O_CLOEXEC)
			if err1 != 0 {
				childExitError(pipe, LocDup3, err1)
			}
			// 设置 close-on-exec 标志
			fd[i] = nextfd
			nextfd++
		}
	}
	// 第二轮文件描述符处理：fd[i] => i
	// 将文件描述符重定向到正确的位置
	for i := 0; i < len(fd); i++ {
		if fd[i] == -1 {
			syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(i), 0, 0)
			continue
		}
		if fd[i] == int(i) {
			// dup2(i, i) 会清除 close-on-exec 标志，因此需要重置标志
			_, _, err1 = syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(fd[i]), syscall.F_SETFD, 0)
			if err1 != 0 {
				childExitError(pipe, LocFcntl, err1)
			}
			continue
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_DUP3, uintptr(fd[i]), uintptr(i), 0)
		if err1 != 0 {
			childExitError(pipe, LocDup3, err1)
		}
	}

	// 设置会话 ID
	_, _, err1 = syscall.RawSyscall(syscall.SYS_SETSID, 0, 0, 0)
	if err1 != 0 {
		childExitError(pipe, LocSetSid, err1)
	}

	// 设置控制终端
	if r.CTTY {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(0), uintptr(syscall.TIOCSCTTY), 1)
		if err1 != 0 {
			childExitError(pipe, LocIoctl, err1)
		}
	}

	// 挂载文件系统
	{
		// 如果挂载点未共享，则标记根目录为私有，以避免传播到原始挂载命名空间
		if r.CloneFlags&syscall.CLONE_NEWNS == syscall.CLONE_NEWNS {
			_, _, err1 = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(&none[0])),
				uintptr(unsafe.Pointer(&slash[0])), 0, syscall.MS_REC|syscall.MS_PRIVATE, 0, 0)
			if err1 != 0 {
				childExitError(pipe, LocMountRoot, err1)
			}
		}

		// 挂载 tmpfs 和 chdir 到新根目录之前执行挂载
		if pivotRoot != nil {
			// 挂载("tmpfs", root, "tmpfs", 0, "")
			_, _, err1 = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(&tmpfs[0])),
				uintptr(unsafe.Pointer(pivotRoot)), uintptr(unsafe.Pointer(&tmpfs[0])), 0,
				uintptr(unsafe.Pointer(&empty[0])), 0)
			if err1 != 0 {
				childExitError(pipe, LocMountTmpfs, err1)
			}

			_, _, err1 = syscall.RawSyscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(pivotRoot)), 0, 0)
			if err1 != 0 {
				childExitError(pipe, LocMountChdir, err1)
			}
		}

		// 执行挂载
		for i, m := range r.Mounts {
			// mkdirs(target)
			for j, p := range m.Prefixes {
				// 如果目标挂载点是文件，则 mknod(target)
				if j == len(m.Prefixes)-1 && m.MakeNod {
					_, _, err1 = syscall.RawSyscall(syscall.SYS_MKNODAT, uintptr(_AT_FDCWD), uintptr(unsafe.Pointer(p)), 0755)
					if err1 != 0 && err1 != syscall.EEXIST {
						childExitErrorWithIndex(pipe, LocMountMkdir, i, err1)
					}
					break
				}
				_, _, err1 = syscall.RawSyscall(syscall.SYS_MKDIRAT, uintptr(_AT_FDCWD), uintptr(unsafe.Pointer(p)), 0755)
				if err1 != 0 && err1 != syscall.EEXIST {
					childExitErrorWithIndex(pipe, LocMountMkdir, i, err1)
				}
			}
			// 挂载(source, target, fsType, flags, data)
			_, _, err1 = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(m.Source)),
				uintptr(unsafe.Pointer(m.Target)), uintptr(unsafe.Pointer(m.FsType)), uintptr(m.Flags),
				uintptr(unsafe.Pointer(m.Data)), 0)
			if err1 != 0 {
				childExitErrorWithIndex(pipe, LocMount, i, err1)
			}
			// 绑定挂载不尊重只读标志，因此需要重新挂载
			if m.Flags&bindRo == bindRo {
				_, _, err1 = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(&empty[0])),
					uintptr(unsafe.Pointer(m.Target)), uintptr(unsafe.Pointer(m.FsType)),
					uintptr(m.Flags|syscall.MS_REMOUNT), uintptr(unsafe.Pointer(m.Data)), 0)
				if err1 != 0 {
					childExitErrorWithIndex(pipe, LocMount, i, err1)
				}
			}
		}

		// pivot_root
		if pivotRoot != nil {
			// mkdir("old_root")
			_, _, err1 = syscall.RawSyscall(syscall.SYS_MKDIRAT, uintptr(_AT_FDCWD), uintptr(unsafe.Pointer(&oldRoot[0])), 0755)
			if err1 != 0 {
				childExitError(pipe, LocPivotRoot, err1)
			}

			// pivot_root(root, "old_root")
			_, _, err1 = syscall.RawSyscall(syscall.SYS_PIVOT_ROOT, uintptr(unsafe.Pointer(pivotRoot)), uintptr(unsafe.Pointer(&oldRoot[0])), 0)
			if err1 != 0 {
				childExitError(pipe, LocPivotRoot, err1)
			}

			// umount("old_root", MNT_DETACH)
			_, _, err1 = syscall.RawSyscall(syscall.SYS_UMOUNT2, uintptr(unsafe.Pointer(&oldRoot[0])), syscall.MNT_DETACH, 0)
			if err1 != 0 {
				childExitError(pipe, LocPivotRoot, err1)
			}

			// rmdir("old_root")
			_, _, err1 = syscall.RawSyscall(syscall.SYS_UNLINKAT, uintptr(_AT_FDCWD), uintptr(unsafe.Pointer(&oldRoot[0])), uintptr(unix.AT_REMOVEDIR))
			if err1 != 0 {
				childExitError(pipe, LocPivotRoot, err1)
			}

			// mount("tmpfs", "/", "tmpfs", MS_BIND | MS_REMOUNT | MS_RDONLY | MS_NOATIME | MS_NOSUID, nil)
			_, _, err1 = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(&tmpfs[0])),
				uintptr(unsafe.Pointer(&slash[0])), uintptr(unsafe.Pointer(&tmpfs[0])),
				uintptr(syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOATIME|syscall.MS_NOSUID),
				uintptr(unsafe.Pointer(&empty[0])), 0)
			if err1 != 0 {
				childExitError(pipe, LocPivotRoot, err1)
			}
		}
	}

	// 设置主机名
	if hostname != nil {
		syscall.RawSyscall(syscall.SYS_SETHOSTNAME,
			uintptr(unsafe.Pointer(hostname)), uintptr(len(r.HostName)), 0)
	}

	// 设置域名
	if domainname != nil {
		syscall.RawSyscall(syscall.SYS_SETDOMAINNAME,
			uintptr(unsafe.Pointer(domainname)), uintptr(len(r.DomainName)), 0)
	}

	// chdir 到工作目录
	if workdir != nil {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(workdir)), 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocChdir, err1)
		}
	}

	// 设置资源限制
	for i, rlim := range r.RLimits {
		// prlimit 代替 setrlimit 以避免 32 位限制（linux > 3.2）
		_, _, err1 = syscall.RawSyscall6(syscall.SYS_PRLIMIT64, 0, uintptr(rlim.Res), uintptr(unsafe.Pointer(&rlim.Rlim)), 0, 0, 0)
		if err1 != 0 {
			childExitErrorWithIndex(pipe, LocSetRlimit, i, err1)
		}
	}

	// 不允许新特权
	if r.NoNewPrivs || r.Seccomp != nil {
		_, _, err1 = syscall.RawSyscall6(syscall.SYS_PRCTL, unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocSetNoNewPrivs, err1)
		}
	}

	// 放弃所有特权
	if (r.Credential != nil || r.DropCaps) && !r.UnshareCgroupAfterSync {
		// 确保子进程没有任何特权
		_, _, err1 = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_SECUREBITS,
			_SECURE_KEEP_CAPS_LOCKED|_SECURE_NO_SETUID_FIXUP|_SECURE_NO_SETUID_FIXUP_LOCKED|_SECURE_NOROOT|_SECURE_NOROOT_LOCKED, 0)
		if err1 != 0 {
			childExitError(pipe, LocDropCapability, err1)
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&dropCapHeader)), uintptr(unsafe.Pointer(&dropCapData)), 0)
		if err1 != 0 {
			childExitError(pipe, LocSetCap, err1)
		}
	}

	// 启用 ptrace 并与父进程同步（因为 ptrace_me 是阻塞操作）
	if r.Ptrace && r.Seccomp != nil {
		{
			r1, _, err1 = syscall.RawSyscall(syscall.SYS_WRITE, uintptr(pipe), uintptr(unsafe.Pointer(&err2)), uintptr(unsafe.Sizeof(err2)))
			if r1 == 0 || err1 != 0 {
				childExitError(pipe, LocSyncWrite, err1)
			}

			r1, _, err1 = syscall.RawSyscall(syscall.SYS_READ, uintptr(pipe), uintptr(unsafe.Pointer(&err2)), uintptr(unsafe.Sizeof(err2)))
			if r1 == 0 || err1 != 0 {
				childExitError(pipe, LocSyncRead, err1)
			}

			// 取消共享 cgroup 命名空间
			if r.UnshareCgroupAfterSync {
				// 如果取消共享失败，不会导致错误
				syscall.RawSyscall(syscall.SYS_UNSHARE, uintptr(unix.CLONE_NEWCGROUP), 0, 0)

				if r.DropCaps || r.Credential != nil {
					// 确保子进程没有任何特权
					_, _, err1 = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_SECUREBITS,
						_SECURE_KEEP_CAPS_LOCKED|_SECURE_NO_SETUID_FIXUP|_SECURE_NO_SETUID_FIXUP_LOCKED|_SECURE_NOROOT|_SECURE_NOROOT_LOCKED, 0)
					if err1 != 0 {
						childExitError(pipe, LocKeepCapability, err1)
					}
					_, _, err1 = syscall.RawSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&dropCapHeader)), uintptr(unsafe.Pointer(&dropCapData)), 0)
					if err1 != 0 {
						childExitError(pipe, LocSetCap, err1)
					}
				}
			}
		}
		_, _, err1 = syscall.RawSyscall(syscall.SYS_PTRACE, uintptr(syscall.PTRACE_TRACEME), 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocPtraceMe, err1)
		}
	}

	// 如果同时定义了 seccomp 和 ptrace，则 seccomp 过滤器应该跟踪 execve，
	// 因此子进程需要父进程附加到它
	// 实际上，如果 pid 命名空间未共享，则无效
	if r.StopBeforeSeccomp || (r.Seccomp != nil && r.Ptrace) {
		// 停止等待 ptrace 跟踪器
		_, _, err1 = syscall.RawSyscall(syscall.SYS_KILL, pid, uintptr(syscall.SIGSTOP), 0)
		if err1 != 0 {
			childExitError(pipe, LocStop, err1)
		}
	}

	// 加载 seccomp 并停止等待跟踪器
	if r.Seccomp != nil && (!r.UnshareCgroupAfterSync || r.Ptrace) {
		// 如果 execve 被 seccomp 跟踪，则需要停止跟踪器
		// 否则 execve 将由于 ENOSYS 失败
		// 执行 getpid 和 kill 以向自身发送 SYS_KILL
		// 需要在 seccomp 之前执行，因为这些可能被跟踪

		// 加载 seccomp 过滤器
		_, _, err1 = syscall.RawSyscall(unix.SYS_SECCOMP, SECCOMP_SET_MODE_FILTER, SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(r.Seccomp)))
		if err1 != 0 {
			childExitError(pipe, LocSeccomp, err1)
		}
	}

	// 在执行前与父进程同步（通过配置为 close-on-exec 的管道）
	if !r.Ptrace || r.Seccomp == nil {
		{
			r1, _, err1 = syscall.RawSyscall(syscall.SYS_WRITE, uintptr(pipe), uintptr(unsafe.Pointer(&err2)), uintptr(unsafe.Sizeof(err2)))
			if r1 == 0 || err1 != 0 {
				childExitError(pipe, LocSyncWrite, err1)
			}

			r1, _, err1 = syscall.RawSyscall(syscall.SYS_READ, uintptr(pipe), uintptr(unsafe.Pointer(&err2)), uintptr(unsafe.Sizeof(err2)))
			if r1 == 0 || err1 != 0 {
				childExitError(pipe, LocSyncRead, err1)
			}

			// 取消共享 cgroup 命名空间
			if r.UnshareCgroupAfterSync {
				// 如果取消共享失败，不会导致错误
				syscall.RawSyscall(syscall.SYS_UNSHARE, uintptr(unix.CLONE_NEWCGROUP), 0, 0)

				if r.DropCaps || r.Credential != nil {
					// 确保子进程没有任何特权
					_, _, err1 = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_SECUREBITS,
						_SECURE_KEEP_CAPS_LOCKED|_SECURE_NO_SETUID_FIXUP|_SECURE_NO_SETUID_FIXUP_LOCKED|_SECURE_NOROOT|_SECURE_NOROOT_LOCKED, 0)
					if err1 != 0 {
						childExitError(pipe, LocKeepCapability, err1)
					}
					_, _, err1 = syscall.RawSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&dropCapHeader)), uintptr(unsafe.Pointer(&dropCapData)), 0)
					if err1 != 0 {
						childExitError(pipe, LocSetCap, err1)
					}
				}

				if r.Seccomp != nil {
					// 加载 seccomp 过滤器
					_, _, err1 = syscall.RawSyscall(unix.SYS_SECCOMP, SECCOMP_SET_MODE_FILTER, SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(r.Seccomp)))
					if err1 != 0 {
						childExitError(pipe, LocSeccomp, err1)
					}
				}
			}
		}
	}

	// 启用 ptrace 如果没有 seccomp
	if r.Ptrace && r.Seccomp == nil {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_PTRACE, uintptr(syscall.PTRACE_TRACEME), 0, 0)
		if err1 != 0 {
			childExitError(pipe, LocPtraceMe, err1)
		}
	}

	// 在此时，runner 已成功附加到 seccomp 跟踪过滤器
	// 或 execve 被跟踪而没有 seccomp 过滤器
	// 现在是执行的时间
	// 如果指定了可执行文件的文件描述符，则调用 fexecve
	if r.ExecFile > 0 {
		_, _, err1 = syscall.RawSyscall6(unix.SYS_EXECVEAT, r.ExecFile,
			uintptr(unsafe.Pointer(&empty[0])), uintptr(unsafe.Pointer(&argv[0])),
			uintptr(unsafe.Pointer(&env[0])), unix.AT_EMPTY_PATH, 0)
	} else {
		_, _, err1 = syscall.RawSyscall(unix.SYS_EXECVE, uintptr(unsafe.Pointer(argv0)),
			uintptr(unsafe.Pointer(&argv[0])), uintptr(unsafe.Pointer(&env[0])))
	}
	// 修复潜在的 ETXTBSY 问题，但要小心（最多 50 次尝试）
	// ETXTBSY 发生在我们将可执行文件复制到容器中时，另一个 goroutine
	// fork 但尚未 execve（设置挂载点需要时间），fork 的进程仍然持有复制的可执行文件的文件描述符
	// 但是，我们不想有不同的逻辑来锁定容器创建
	for range [50]struct{}{} {
		if err1 != syscall.ETXTBSY {
			break
		}
		// 等待而不是忙等
		syscall.RawSyscall(unix.SYS_NANOSLEEP, uintptr(unsafe.Pointer(&etxtbsyRetryInterval)), 0, 0)
		if r.ExecFile > 0 {
			_, _, err1 = syscall.RawSyscall6(unix.SYS_EXECVEAT, r.ExecFile,
				uintptr(unsafe.Pointer(&empty[0])), uintptr(unsafe.Pointer(&argv[0])),
				uintptr(unsafe.Pointer(&env[0])), unix.AT_EMPTY_PATH, 0)
		} else {
			_, _, err1 = syscall.RawSyscall(unix.SYS_EXECVE, uintptr(unsafe.Pointer(argv0)),
				uintptr(unsafe.Pointer(&argv[0])), uintptr(unsafe.Pointer(&env[0])))
		}
	}
	childExitError(pipe, LocExecve, err1)
	return
}

//go:nosplit
func childExitError(pipe int, loc ErrorLocation, err syscall.Errno) {
	// 发送错误代码到管道
	childError := ChildError{
		Err:      err,
		Location: loc,
	}

	// 发送错误代码到管道
	syscall.RawSyscall(unix.SYS_WRITE, uintptr(pipe), uintptr(unsafe.Pointer(&childError)), unsafe.Sizeof(childError))
	for {
		syscall.RawSyscall(syscall.SYS_EXIT, uintptr(err), 0, 0)
	}
}

//go:nosplit
func childExitErrorWithIndex(pipe int, loc ErrorLocation, idx int, err syscall.Errno) {
	// 发送错误代码到管道
	childError := ChildError{
		Err:      err,
		Location: loc,
		Index:    idx,
	}

	// 发送错误代码到管道
	syscall.RawSyscall(unix.SYS_WRITE, uintptr(pipe), uintptr(unsafe.Pointer(&childError)), unsafe.Sizeof(childError))
	for {
		syscall.RawSyscall(syscall.SYS_EXIT, uintptr(err), 0, 0)
	}
}
