// Package unshare 实现了基于 Linux namespace 隔离的沙箱
package unshare

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/zqzqsb/sandbox/pkg/forkexec"
	"github.com/zqzqsb/sandbox/runner"
)

const (
	// UnshareFlags 定义了创建新命名空间的标志位组合
	// 包括：
	// - CLONE_NEWNS: 挂载命名空间隔离
	// - CLONE_NEWPID: 进程 ID 命名空间隔离
	// - CLONE_NEWUSER: 用户命名空间隔离
	// - CLONE_NEWUTS: UTS(主机名和域名)命名空间隔离
	// - CLONE_NEWCGROUP: Cgroup 命名空间隔离
	// 注意：不包括网络和 IPC 命名空间
	UnshareFlags = unix.CLONE_NEWNS | unix.CLONE_NEWPID | unix.CLONE_NEWUSER | unix.CLONE_NEWUTS | unix.CLONE_NEWCGROUP
)

// Run 启动一个带有命名空间隔离的进程
// 参数 c 用于控制进程的生命周期
func (r *Runner) Run(c context.Context) (result runner.Result) {
	// 配置 forkexec 运行器
	ch := &forkexec.Runner{
		Args:       r.Args,       // 命令行参数
		Env:        r.Env,        // 环境变量
		ExecFile:   r.ExecFile,   // 可执行文件路径
		RLimits:    r.RLimits,    // 资源限制
		Files:      r.Files,      // 文件描述符
		WorkDir:    r.WorkDir,    // 工作目录
		Seccomp:    r.Seccomp.SockFprog(), // Seccomp 过滤器
		NoNewPrivs: true,         // 禁止获取新特权
		CloneFlags: UnshareFlags, // 命名空间隔离标志
		Mounts:     r.Mounts,     // 挂载点配置
		HostName:   r.HostName,   // 主机名
		DomainName: r.DomainName, // 域名
		PivotRoot:  r.Root,       // 根目录切换
		DropCaps:   true,         // 移除特权
		SyncFunc:   r.SyncFunc,   // 同步函数

		UnshareCgroupAfterSync: true, // 同步后再隔离 Cgroup
	}

	var (
		wstatus unix.WaitStatus // wait4 系统调用的等待状态
		rusage  unix.Rusage     // wait4 系统调用的资源使用统计
		status  = runner.StatusNormal // 进程状态
		sTime   = time.Now()    // 启动时间
		fTime   time.Time       // 设置完成时间
	)

	// 启动进程并获取进程组 ID
	pgid, err := ch.Start()
	r.println("Starts: ", pgid, err)
	if err != nil {
		result.Status = runner.StatusRunnerError
		result.Error = err.Error()
		return
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(c)
	defer cancel()

	// 处理取消信号
	go func() {
		<-ctx.Done()
		killAll(pgid) // 收到取消信号时终止所有进程
	}()

	// 确保在函数返回时清理所有子进程
	defer func() {
		killAll(pgid)      // 终止所有进程
		collectZombie(pgid) // 回收僵尸进程
		result.SetUpTime = fTime.Sub(sTime)    // 记录设置耗时
		result.RunningTime = time.Since(fTime) // 记录运行耗时
	}()

	fTime = time.Now()
	for {
		// 等待任意子进程状态改变
		_, err := unix.Wait4(pgid, &wstatus, 0, &rusage)
		if err == unix.EINTR {
			continue // 被信号中断时继续等待
		}
		r.println("wait4: ", wstatus)
		if err != nil {
			result.Status = runner.StatusRunnerError
			result.Error = err.Error()
			return
		}

		// 更新资源使用统计
		userTime := time.Duration(rusage.Utime.Nano()) // 用户态 CPU 时间（纳秒）
		userMem := runner.Size(rusage.Maxrss << 10)    // 最大常驻内存（字节）

		// 检查是否超出资源限制
		if userTime > r.Limit.TimeLimit {
			status = runner.StatusTimeLimitExceeded // 超时
		}
		if userMem > r.Limit.MemoryLimit {
			status = runner.StatusMemoryLimitExceeded // 超内存
		}
		result = runner.Result{
			Status: status,
			Time:   userTime,
			Memory: userMem,
		}
		if status != runner.StatusNormal {
			return
		}

		switch {
		case wstatus.Exited(): // 进程正常退出
			result.Status = runner.StatusNormal
			result.ExitStatus = wstatus.ExitStatus()
			if result.ExitStatus != 0 {
				result.Status = runner.StatusNonzeroExitStatus // 非零退出状态
			}
			return

		case wstatus.Signaled(): // 进程被信号终止
			sig := wstatus.Signal()
			switch sig {
			case unix.SIGXCPU, unix.SIGKILL:
				status = runner.StatusTimeLimitExceeded // CPU 时间限制或被强制终止
			case unix.SIGXFSZ:
				status = runner.StatusOutputLimitExceeded // 文件大小限制
			case unix.SIGSYS:
				status = runner.StatusDisallowedSyscall // 禁止的系统调用
			default:
				status = runner.StatusSignalled // 其他信号终止
			}
			result.Status = status
			result.ExitStatus = int(sig)
			return
		}
	}
}

// killAll 终止指定进程组的所有进程
func killAll(pgid int) {
	unix.Kill(-pgid, unix.SIGKILL) // 负数 pgid 表示发送信号给整个进程组
}

// collectZombie 回收指定进程组的所有僵尸进程
func collectZombie(pgid int) {
	var wstatus unix.WaitStatus
	for {
		// WALL: 等待所有子进程
		// WNOHANG: 非阻塞等待
		if _, err := unix.Wait4(-pgid, &wstatus, unix.WALL|unix.WNOHANG, nil); err != unix.EINTR && err != nil {
			break
		}
	}
}

// println 输出调试信息到标准错误
func (r *Runner) println(v ...interface{}) {
	if r.ShowDetails {
		fmt.Fprintln(os.Stderr, v...)
	}
}
