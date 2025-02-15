package forkexec

import (
	"syscall"
	"unsafe" // 需要用于 go:linkname

	"golang.org/x/sys/unix"
)

// Start 函数会执行以下操作：
// 1. fork 创建子进程
// 2. 加载 seccomp 安全策略
// 3. 执行 execve 系统调用
// 4. 如果开启了 ptrace，进程会被跟踪
//
// 返回值：
// - pid: 子进程的进程ID
// - error: 可能的错误
//
// 注意：如果启用了 ptrace，在调用此函数前必须锁定当前 OS 线程
func (r *Runner) Start() (int, error) {
	// 准备执行参数：程序路径、参数列表和环境变量
	argv0, argv, env, err := prepareExec(r.Args, r.Env)
	if err != nil {
		return 0, err
	}

	// 准备工作目录路径
	workdir, err := syscallStringFromString(r.WorkDir)
	if err != nil {
		return 0, err
	}

	// 准备主机名
	hostname, err := syscallStringFromString(r.HostName)
	if err != nil {
		return 0, err
	}

	// 准备域名
	domainname, err := syscallStringFromString(r.DomainName)
	if err != nil {
		return 0, err
	}

	// 准备根目录切换参数
	pivotRoot, err := syscallStringFromString(r.PivotRoot)
	if err != nil {
		return 0, err
	}

	// 创建一对 socket 用于父子进程通信
	// p[0] 由父进程使用，p[1] 由子进程使用
	// 用途：
	// 1. 通知子进程 uid/gid 映射已经设置完成
	// 2. 在最终 execve 之前与父进程同步
	p, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}

	// 在子进程中执行 fork 和 exec
	pid, err1 := forkAndExecInChild(r, argv0, argv, env, workdir, hostname, domainname, pivotRoot, p)

	// 恢复所有信号处理
	afterFork()
	syscall.ForkLock.Unlock()

	// 与子进程同步并处理结果
	return syncWithChild(r, p, int(pid), err1)
}

// syncWithChild 函数负责父进程与子进程的同步操作
// 主要完成以下工作：
// 1. 设置 uid/gid 映射（如果启用了用户命名空间）
// 2. 处理子进程返回的错误
// 3. 执行用户定义的同步函数
// 4. 处理 ptrace 相关的同步
func syncWithChild(r *Runner, p [2]int, pid int, err1 syscall.Errno) (int, error) {
	var (
		err2        syscall.Errno
		err         error
		unshareUser = r.CloneFlags&unix.CLONE_NEWUSER == unix.CLONE_NEWUSER // 检查是否启用了用户命名空间
		childErr    ChildError
	)

	// 关闭子进程端的管道
	unix.Close(p[1])

	// 如果 clone 系统调用失败，直接返回错误
	if err1 != 0 {
		unix.Close(p[0])
		childErr.Location = LocClone
		childErr.Err = err1
		return 0, childErr
	}

	// 如果启用了用户命名空间，需要设置 uid/gid 映射
	if unshareUser {
		if err = writeIDMaps(r, int(pid)); err != nil {
			err2 = err.(syscall.Errno)
		}
		// 通知子进程 uid/gid 映射已完成
		syscall.RawSyscall(syscall.SYS_WRITE, uintptr(p[0]), uintptr(unsafe.Pointer(&err2)), uintptr(unsafe.Sizeof(err2)))
	}

	// 读取子进程可能返回的错误
	n, err := readChildErr(p[0], &childErr)
	// 如果读取的数据大小不对，或者子进程返回了错误，则处理失败情况
	if (n != int(unsafe.Sizeof(err2)) && n != int(unsafe.Sizeof(childErr))) || childErr.Err != 0 || err != nil {
		childErr.Err = handlePipeError(n, childErr.Err)
		goto fail
	}

	// 执行用户定义的同步函数（如果有）
	if r.SyncFunc != nil {
		if err = r.SyncFunc(int(pid)); err != nil {
			goto fail
		}
	}
	// 向子进程发送确认信号
	syscall.RawSyscall(syscall.SYS_WRITE, uintptr(p[0]), uintptr(unsafe.Pointer(&err1)), uintptr(unsafe.Sizeof(err1)))

	// 如果进程需要在 execve 之前停止（因为 ptrace 或 StopBeforeSeccomp）
	if r.Ptrace || r.StopBeforeSeccomp {
		// 在另一个 goroutine 中等待，避免 SIGPIPE
		go func() {
			readChildErr(p[0], &childErr)
			unix.Close(p[0])
		}()
		return int(pid), nil
	}

	// 检查子进程在同步后是否失败
	n, err = readChildErr(p[0], &childErr)
	unix.Close(p[0])
	if n != 0 || err != nil {
		childErr.Err = handlePipeError(n, childErr.Err)
		goto failAfterClose
	}
	return int(pid), nil

fail:
	unix.Close(p[0])

failAfterClose:
	handleChildFailed(int(pid))
	if childErr.Err == 0 {
		return 0, err
	}
	return 0, childErr
}

// readChildErr 从文件描述符中读取子进程的错误信息
// 如果被 EINTR 信号中断，会重试读取操作
func readChildErr(fd int, childErr *ChildError) (n int, err error) {
	for {
		n, err = readlen(fd, (*byte)(unsafe.Pointer(childErr)), int(unsafe.Sizeof(*childErr)))
		if err != syscall.EINTR {
			break
		}
	}
	return
}

// readlen 是一个底层的读取函数，直接调用 read 系统调用
// 用于从文件描述符读取指定长度的数据
func readlen(fd int, p *byte, np int) (n int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(np))
	n = int(r0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// handlePipeError 处理管道错误
// 如果读取的数据长度足够，返回实际的错误码
// 否则返回 EPIPE 错误
func handlePipeError(r1 int, errno syscall.Errno) syscall.Errno {
	if uintptr(r1) >= unsafe.Sizeof(errno) {
		return syscall.Errno(errno)
	}
	return syscall.EPIPE
}

// handleChildFailed 处理子进程失败的情况
// 1. 向子进程发送 SIGKILL 信号确保其终止
// 2. 等待子进程退出，避免产生僵尸进程
func handleChildFailed(pid int) {
	var wstatus syscall.WaitStatus
	// 确保子进程被终止
	syscall.Kill(pid, syscall.SIGKILL)
	// 等待子进程退出，避免僵尸进程
	_, err := syscall.Wait4(pid, &wstatus, 0, nil)
	for err == syscall.EINTR {
		_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	}
}
