// Package ptrace 提供了基于 ptrace 的系统调用跟踪和控制功能
package ptrace

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/zqzqsb/sandbox/pkg/seccomp/libseccomp"
	"github.com/zqzqsb/sandbox/ptracer"
)

// tracerHandler 实现了系统调用跟踪和处理的核心逻辑
type tracerHandler struct {
	ShowDetails bool    // 是否显示详细的调试信息
	Unsafe      bool    // 是否启用不安全模式（软禁用而不是直接杀死进程）
	Handler     Handler // 具体的系统调用处理器
}

// Debug 输出调试信息到标准错误输出
// 只有在 ShowDetails 为 true 时才会输出
func (h *tracerHandler) Debug(v ...interface{}) {
	if h.ShowDetails {
		fmt.Fprintln(os.Stderr, v...)
	}
}

// getString 从目标进程的内存中读取字符串
// ctx: ptrace 上下文
// addr: 目标进程中字符串的地址
// 返回: 转换为绝对路径的字符串
func (h *tracerHandler) getString(ctx *ptracer.Context, addr uint) string {
	return absPath(ctx.Pid, ctx.GetString(uintptr(addr)))
}

// checkOpen 检查打开文件的操作是否允许
// ctx: ptrace 上下文
// addr: 文件路径在目标进程中的地址
// flags: 打开文件的标志位
// 返回: 跟踪动作（允许/禁止/杀死）
func (h *tracerHandler) checkOpen(ctx *ptracer.Context, addr uint, flags uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	// 判断是否为只读操作
	isReadOnly := (flags&syscall.O_ACCMODE == syscall.O_RDONLY) &&
		(flags&syscall.O_CREAT == 0) &&
		(flags&syscall.O_EXCL == 0) &&
		(flags&syscall.O_TRUNC == 0)

	h.Debug("open: ", fn, getFileMode(flags))
	if isReadOnly {
		return h.Handler.CheckRead(fn)
	}
	return h.Handler.CheckWrite(fn)
}

// checkRead 检查读取文件的操作是否允许
func (h *tracerHandler) checkRead(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check read: ", fn)
	return h.Handler.CheckRead(fn)
}

// checkWrite 检查写入文件的操作是否允许
func (h *tracerHandler) checkWrite(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check write: ", fn)
	return h.Handler.CheckWrite(fn)
}

// checkStat 检查获取文件状态的操作是否允许
func (h *tracerHandler) checkStat(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check stat: ", fn)
	return h.Handler.CheckStat(fn)
}

// Handle 处理系统调用
// 这是 ptrace 跟踪器的主要处理函数，它：
// 1. 识别系统调用号
// 2. 根据系统调用类型分发到相应的处理函数
// 3. 返回适当的跟踪动作
func (h *tracerHandler) Handle(ctx *ptracer.Context) ptracer.TraceAction {
	syscallNo := ctx.SyscallNo()
	syscallName, err := libseccomp.ToSyscallName(syscallNo)
	h.Debug("syscall:", syscallNo, syscallName, err)
	if err != nil {
		h.Debug("invalid syscall no")
		return ptracer.TraceKill
	}

	action := ptracer.TraceKill
	switch syscallName {
	// 文件打开相关系统调用
	case "open":
		action = h.checkOpen(ctx, ctx.Arg0(), ctx.Arg1())
	case "openat":
		action = h.checkOpen(ctx, ctx.Arg1(), ctx.Arg2())

	// 符号链接读取相关系统调用
	case "readlink":
		action = h.checkRead(ctx, ctx.Arg0())
	case "readlinkat":
		action = h.checkRead(ctx, ctx.Arg1())

	// 文件删除相关系统调用
	case "unlink":
		action = h.checkWrite(ctx, ctx.Arg0())
	case "unlinkat":
		action = h.checkWrite(ctx, ctx.Arg1())

	// 文件访问权限检查相关系统调用
	case "access":
		action = h.checkStat(ctx, ctx.Arg0())
	case "faccessat", "newfstatat":
		action = h.checkStat(ctx, ctx.Arg1())

	// 文件状态查询相关系统调用
	case "stat", "stat64":
		action = h.checkStat(ctx, ctx.Arg0())
	case "lstat", "lstat64":
		action = h.checkStat(ctx, ctx.Arg0())

	// 程序执行相关系统调用
	case "execve":
		action = h.checkRead(ctx, ctx.Arg0())
	case "execveat":
		action = h.checkRead(ctx, ctx.Arg1())

	// 文件权限修改相关系统调用
	case "chmod":
		action = h.checkWrite(ctx, ctx.Arg0())
	case "rename":
		action = h.checkWrite(ctx, ctx.Arg0())

	// 其他系统调用
	default:
		action = h.Handler.CheckSyscall(syscallName)
		if h.Unsafe && action == ptracer.TraceKill {
			action = ptracer.TraceBan
		}
	}

	// 根据动作类型返回相应的处理结果
	switch action {
	case ptracer.TraceAllow:
		return ptracer.TraceAllow
	case ptracer.TraceBan:
		h.Debug("<soft ban syscall>")
		return softBanSyscall(ctx)
	default:
		return ptracer.TraceKill
	}
}

// softBanSyscall 软禁用系统调用
// 不是直接杀死进程，而是返回一个错误码
func softBanSyscall(ctx *ptracer.Context) ptracer.TraceAction {
	ctx.SetReturnValue(-int(BanRet))
	return ptracer.TraceBan
}

// getFileMode 获取文件打开模式的字符串表示
// flags: 打开文件的标志位
// 返回: 模式字符串 ("r "表示只读, "w "表示只写, "wr"表示读写)
func getFileMode(flags uint) string {
	switch flags & syscall.O_ACCMODE {
	case syscall.O_RDONLY:
		return "r "
	case syscall.O_WRONLY:
		return "w "
	case syscall.O_RDWR:
		return "wr"
	default:
		return "??"
	}
}

// getProcCwd 获取进程的当前工作目录
// pid: 进程ID，如果为0则表示当前进程
// 返回: 工作目录的路径，如果出错则返回空字符串
func getProcCwd(pid int) string {
	fileName := "/proc/self/cwd"
	if pid > 0 {
		fileName = fmt.Sprintf("/proc/%d/cwd", pid)
	}
	s, err := os.Readlink(fileName)
	if err != nil {
		return ""
	}
	return s
}

// absPath 计算进程相对的绝对路径
// pid: 进程ID
// p: 原始路径
// 返回: 转换后的绝对路径
func absPath(pid int, p string) string {
	// 如果不是绝对路径，则基于进程的工作目录计算
	if !path.IsAbs(p) {
		return path.Join(getProcCwd(pid), p)
	}
	return path.Clean(p)
}
