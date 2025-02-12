package ptracer

import (
	"context"
	"fmt"
	"runtime"
	"time"

	unix "golang.org/x/sys/unix"

	"github.com/zqzqsb/sandbox/runner"
)

/*
	Trace 启动并跟踪目标进程及其子进程

Trace 在当前 goroutine 中启动一个受控的进程，并对其进行跟踪。它使用 ptrace
机制来监控进程的执行，包括系统调用、信号处理和资源使用情况。

实现细节：
 1. 锁定当前线程以确保 ptrace 操作的稳定性
 2. 通过 Runner 接口启动目标进程
 3. 建立进程组跟踪关系
 4. 处理启动过程中的错误情况

参数：
  - c: 上下文对象，用于取消操作和控制跟踪生命周期

返回值：
  - result: 包含进程执行的最终状态、资源使用情况和错误信息

错误处理：
 1. 进程启动失败：返回 StatusRunnerError 状态
 2. 运行时错误：通过 Debug 接口记录详细信息
 3. 取消操作：通过 context 实现优雅终止

注意事项：
 1. 该函数会锁定调用它的 goroutine 到特定的操作系统线程
 2. 在整个跟踪过程中必须保持线程锁定
 3. 确保 Runner.Start() 正确设置了 ptrace 跟踪标志
*/
func (t *Tracer) Trace(c context.Context) (result runner.Result) {
	// ptrace 是基于线程的（内核进程）
	// Goroutine 1 -----> OS Thread 1  -----> Child Process
	//                   (locked)            (being traced)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 启动运行器(子进程)
	pgid, err := t.Runner.Start()
	t.Handler.Debug("tracer started:", pgid, err)
	if err != nil {
		t.Handler.Debug("failed to start traced process:", err)
		result.Status = runner.StatusRunnerError
		result.Error = err.Error()
		return
	}
	return t.trace(c, pgid)
}

/* trace 实现进程跟踪的核心逻辑

trace 函数实现了对目标进程及其子进程的具体跟踪逻辑，包括资源限制检查、
信号处理和系统调用跟踪等功能。

实现细节：
  1. 创建取消上下文，用于优雅终止
  2. 设置资源使用监控
  3. 处理进程状态变化
  4. 管理多进程跟踪

参数：
  - c: 上下文对象，用于取消操作
  - pgid: 进程组ID，用于跟踪整个进程组

返回值：
  - result: 包含进程执行的最终状态和资源使用情况

状态处理：
  1. 正常退出：返回进程的退出状态
  2. 异常终止：返回对应的错误状态
  3. 资源超限：返回相应的限制状态 */

/* trace 实现进程跟踪的核心循环

trace 函数实现了对目标进程及其子进程的具体跟踪逻辑，包括：
  1. 资源限制检查
  2. 信号处理
  3. 系统调用跟踪
  4. 进程状态管理
  5. 错误恢复机制

函数流程：
  1. 初始化上下文和清理函数
  2. 创建进程跟踪处理器
  3. 进入 ptrace 主循环
  4. 处理各种进程状态和信号
  5. 收集资源使用情况 */

func (t *Tracer) trace(c context.Context, pgid int) (result runner.Result) {
	// 创建可取消的子上下文，用于控制跟踪过程
	cc, cancel := context.WithCancel(c)
	defer cancel()

	// 启动 goroutine 监听取消信号
	// 当上下文被取消时，终止所有相关进程
	go func() {
		<-cc.Done()
		killAll(pgid)
	}()

	// 记录开始时间，用于计算设置时间和运行时间
	sTime := time.Now()
	// 创建进程跟踪处理器，管理进程状态
	ph := newPtraceHandle(t, pgid)

	// 设置 defer 函数处理 panic 和清理工作
	defer func() {
		// 捕获并处理可能的 panic
		if err := recover(); err != nil {
			t.Handler.Debug("panic occurred:", err)
			result.Status = runner.StatusRunnerError
			result.Error = fmt.Sprintf("%v", err)
		}
		// 清理所有进程
		killAll(pgid)
		// 回收僵尸进程
		collectZombie(pgid)
		// 计算时间统计
		if !ph.fTime.IsZero() {
			// 设置时间：从开始到第一个进程执行
			result.SetUpTime = ph.fTime.Sub(sTime)
			// 运行时间：从第一个进程执行到现在
			result.RunningTime = time.Since(ph.fTime)
		}
	}()

	// ptrace 主循环：等待和处理进程事件
	for {
		var (
			wstatus unix.WaitStatus // 进程状态
			rusage  unix.Rusage     // 资源使用统计
			pid     int             // 触发事件的进程 ID
			err     error
		)

		/*
			unix.Wait4(pid, wstatus, options, rusage)
			pid 参数的不同值有不同含义：
			< -1  : 等待进程组 ID 为 |pid| 的任何子进程
			-1    : 等待任何子进程
			 0    : 等待调用进程的进程组中的任何子进程
			> 0   : 等待特定 PID 的进程
		*/
		// 根据是否已执行 exec 选择不同的等待策略
		if ph.execved {
			// exec 后等待进程组中的任意子进程
			pid, err = unix.Wait4(-pgid, &wstatus, unix.WALL, &rusage)
		} else {
			// exec 前只等待主进程
			pid, err = unix.Wait4(pgid, &wstatus, unix.WALL, &rusage)
		}

		// 处理等待过程中的错误
		if err == unix.EINTR {
			// 等待被信号中断，继续等待
			t.Handler.Debug("wait4 interrupted")
			continue
		}
		if err != nil {
			// 等待出错，返回错误状态
			t.Handler.Debug("wait4 failed:", err)
			result.Status = runner.StatusRunnerError
			result.Error = err.Error()
			return
		}
		t.Handler.Debug("------ process:", pid, "------")

		// 对主进程进行资源使用检查
		if pid == pgid {
			// 检查 CPU 时间、内存使用等
			userTime, userMem, curStatus := t.checkUsage(rusage)
			result.Status = curStatus
			result.Time = userTime
			result.Memory = userMem
			// 如果资源超限，立即返回
			if curStatus != runner.StatusNormal {
				return
			}
		}

		// 处理进程状态变化
		status, exitStatus, errStr, finished := ph.handle(pid, wstatus)
		if finished || status != runner.StatusNormal {
			// 设置结果并返回
			result.Status = status
			result.ExitStatus = exitStatus
			result.Error = errStr
			return
		}
	}
}

func (t *Tracer) checkUsage(rusage unix.Rusage) (time.Duration, runner.Size, runner.Status) {
	status := runner.StatusNormal
	// 更新资源使用情况并检查是否超过限制
	userTime := time.Duration(rusage.Utime.Nano()) // 纳秒
	userMem := runner.Size(rusage.Maxrss << 10)    // 字节

	// 检查是否超时/超内存
	if userTime > t.Limit.TimeLimit {
		status = runner.StatusTimeLimitExceeded
	}
	if userMem > t.Limit.MemoryLimit {
		status = runner.StatusMemoryLimitExceeded
	}
	return userTime, userMem, status
}

/*
	进程状态处理函数

handle 函数负责处理进程状态变化，包括进程退出、信号终止和停止等情况。
它根据进程状态进行相应的处理，并返回当前状态、退出状态和错误信息。

参数：
  - pid: 进程 ID
  - wstatus: 进程状态

返回值：
  - status: 当前状态
  - exitStatus: 退出状态
  - errStr: 错误信息
  - finished: 是否完成
*/

func (ph *ptraceHandle) handle(pid int, wstatus unix.WaitStatus) (status runner.Status, exitStatus int, errStr string, finished bool) {
	// 默认状态为正常
	status = runner.StatusNormal

	// 根据进程的状态变化进行处理
	switch {
	// 1. 进程正常退出的情况
	case wstatus.Exited():
		// 从跟踪表中移除该进程
		delete(ph.traced, pid)
		ph.Handler.Debug("process exited:", pid, "status:", wstatus.ExitStatus())
		
		// 如果是主进程退出
		if pid == ph.pgid {
			finished = true
			// 如果已经执行过 exec
			if ph.execved {
				exitStatus = wstatus.ExitStatus()
				if exitStatus == 0 {
					// 正常退出，状态码为0
					status = runner.StatusNormal
				} else {
					// 异常退出，状态码非0
					status = runner.StatusNonzeroExitStatus
				}
				return
			}
			// exec 前就退出，说明启动失败
			status = runner.StatusRunnerError
			errStr = "child process exited before execve"
			return
		}

	// 2. 进程被信号终止的情况
	case wstatus.Signaled():
		sig := wstatus.Signal()
		// 从跟踪表中移除该进程
		delete(ph.traced, pid)
		ph.Handler.Debug("process terminated by signal:", pid, "signal:", sig)
		
		// 如果是主进程被信号终止
		if pid == ph.pgid {
			finished = true
			status = runner.StatusSignalled
			errStr = fmt.Sprintf("process killed by signal %d", sig)
			return
		}

	// 3. 进程停止的情况
	case wstatus.Stopped():
		// 如果是新的进程（首次停止）
		if !ph.traced[pid] {
			// 添加到跟踪表
			ph.traced[pid] = true
			ph.Handler.Debug("start tracing process:", pid)
			
			// 设置 ptrace 选项，启用各种事件通知
			err := setPtraceOption(pid)
			if err != nil {
				ph.Handler.Debug("failed to set ptrace options:", err)
				status = runner.StatusRunnerError
				errStr = fmt.Sprintf("failed to set ptrace options: %v", err)
				return
			}
			
			// 如果是主进程且尚未执行 exec，记录开始时间
			if !ph.execved && pid == ph.pgid {
				ph.fTime = time.Now()
			}
		}

		// 获取导致进程停止的信号
		sig := wstatus.StopSignal()
		// 如果是 SIGTRAP（跟踪陷阱）
		if sig == unix.SIGTRAP {
			// 获取具体的事件类型
			event := ((wstatus.TrapCause()) >> 8) & 255
			
			// 3.1 seccomp 系统调用过滤器触发
			if event == unix.PTRACE_EVENT_SECCOMP {
				if err := ph.handleTrap(pid); err != nil {
					ph.Handler.Debug("failed to handle seccomp trap:", err)
					status = runner.StatusRunnerError
					errStr = fmt.Sprintf("failed to handle seccomp trap: %v", err)
					return
				}
			// 3.2 exec 事件：进程执行了新程序
			} else if event == unix.PTRACE_EVENT_EXEC {
				ph.Handler.Debug("process exec event:", pid)
				ph.execved = true
			// 3.3 进程创建事件：clone/fork/vfork
			} else if event == unix.PTRACE_EVENT_CLONE ||
				event == unix.PTRACE_EVENT_VFORK ||
				event == unix.PTRACE_EVENT_FORK {
				ph.Handler.Debug("process clone/fork event:", pid)
			// 3.4 其他 trap 事件
			} else {
				ph.Handler.Debug("process trap:", pid, "event:", event)
			}
			// 清除信号，因为这是跟踪事件，不需要传递给进程
			sig = 0
		}
		
		// 继续运行进程，传递适当的信号
		err := unix.PtraceCont(pid, int(sig))
		if err != nil {
			ph.Handler.Debug("failed to continue process:", err)
			status = runner.StatusRunnerError
			errStr = fmt.Sprintf("failed to continue process: %v", err)
			return
		}
	}
	return
}

/*
	handleTrap 处理 seccomp 系统调用陷阱

handleTrap 函数负责处理由 seccomp 过滤器触发的系统调用陷阱。它获取系统调用
上下文，并根据处理器的决定执行相应的操作。

实现细节：
 1. 获取系统调用上下文
 2. 调用自定义处理器
 3. 根据处理结果执行对应操作

参数：
  - pid: 触发陷阱的进程ID

返回值：
  - error: 处理过程中的错误，nil 表示成功

处理动作：
 1. TraceBan: 跳过系统调用
 2. TraceKill: 终止进程
 3. TraceAllow: 允许系统调用继续执行
*/
func (ph *ptraceHandle) handleTrap(pid int) error {
	ph.Handler.Debug("seccomp trap occurred")
	// msg, err := unix.PtraceGetEventMsg(pid)
	// if err != nil {
	// 	t.Handler.Debug("PtraceGetEventMsg failed:", err)
	// 	return err
	// }
	if ph.Handler != nil {
		ctx, err := getTrapContext(pid)
		if err != nil {
			return err
		}
		act := ph.Handler.Handle(ctx)

		switch act {
		case TraceBan:
			// 将系统调用号设置为-1并将返回值写入寄存器以跳过系统调用
			// https://www.kernel.org/doc/Documentation/prctl/pkg/seccomp_filter.txt
			return ctx.skipSyscall()

		case TraceKill:
			return runner.StatusDisallowedSyscall
		}
	}
	return nil
}

// setPtraceOption 设置Ptrace选项，包括seccomp、退出时终止和所有多进程操作
func setPtraceOption(pid int) error {
	// 设置子进程退出时终止
	if err := unix.PtraceSetOptions(pid, unix.PTRACE_O_EXITKILL|
		unix.PTRACE_O_TRACECLONE|unix.PTRACE_O_TRACEFORK|unix.PTRACE_O_TRACEVFORK|
		unix.PTRACE_O_TRACEEXEC|unix.PTRACE_O_TRACESECCOMP|unix.PTRACE_O_TRACEEXIT); err != nil {
		return fmt.Errorf("failed to set ptrace options: %v", err)
	}
	return nil
}

// killAll 根据进程组ID终止所有被跟踪的进程
func killAll(pgid int) {
	unix.Kill(-pgid, unix.SIGKILL)
}

// collectZombie 收集已终止的子进程
func collectZombie(pgid int) {
	var (
		wstatus unix.WaitStatus
		rusage  unix.Rusage
	)
	for {
		// 等待任何子进程，不阻塞
		pid, err := unix.Wait4(-pgid, &wstatus, unix.WALL|unix.WNOHANG, &rusage)
		if err != nil || pid <= 0 {
			return
		}
	}
}

/*
字段说明：
  *Tracer: 嵌入的跟踪器对象，继承其所有方法
  pgid: 进程组ID，用于标识和管理整个进程组
  traced: 记录所有被跟踪的进程
  execved: 标记是否已执行过 exec
  fTime: 第一个进程的启动时间
*/

type ptraceHandle struct {
	*Tracer
	pgid    int
	traced  map[int]bool
	execved bool
	fTime   time.Time
}

func newPtraceHandle(t *Tracer, pgid int) *ptraceHandle {
	return &ptraceHandle{t, pgid, make(map[int]bool), false, time.Time{}}
}
