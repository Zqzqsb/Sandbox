package ptracer

import (
	"syscall"
)

/*
	; x86_64 系统调用参数顺序
	syscall_number -> rax    ; 系统调用号
	arg0 -> rdi             ; 第1个参数
	arg1 -> rsi             ; 第2个参数
	arg2 -> rdx             ; 第3个参数
	arg3 -> r10            ; 第4个参数（注意：不是 rcx）
	arg4 -> r8             ; 第5个参数
	arg5 -> r9             ; 第6个参数
*/

// SyscallNo 获取当前系统调用号
func (c *Context) SyscallNo() uint {
	return uint(c.regs.Orig_rax)  // 使用 Orig_rax 而不是 rax
    // 因为 rax 会被系统调用返回值覆盖 Orig_rax 是一个特殊的寄存器值，用于保存系统调用的原始号码。
}

// Arg0 获取当前系统调用的 arg0
func (c *Context) Arg0() uint {
	return uint(c.regs.Rdi)
}

// Arg1 获取当前系统调用的 arg1
func (c *Context) Arg1() uint {
	return uint(c.regs.Rsi)
}

// Arg2 获取当前系统调用的 arg2
func (c *Context) Arg2() uint {
	return uint(c.regs.Rdx)
}

// Arg3 获取当前系统调用的 arg3
func (c *Context) Arg3() uint {
	return uint(c.regs.R10)
}

// Arg4 获取当前系统调用的 arg4
func (c *Context) Arg4() uint {
	return uint(c.regs.R8)
}

// Arg5 获取当前系统调用的 arg5
func (c *Context) Arg5() uint {
	return uint(c.regs.R9)
}

// SetReturnValue 在跳过系统调用时设置返回值
func (c *Context) SetReturnValue(retval int) {
	c.regs.Rax = uint64(retval)
}

// skipSyscall 跳过当前系统调用
// 通过将系统调用号设置为 -1 (^uint64(0))，使系统调用返回 ENOSYS 错误
// 这是一个常用技巧，用于：
// 1. 阻止系统调用实际执行
// 2. 让进程收到 "系统调用不存在" 的错误
// 3. 在不终止进程的情况下拦截系统调用
func (c *Context) skipSyscall() error {
    c.regs.Orig_rax = ^uint64(0)  // 设置为 -1，在无符号整数中表示最大值
    return syscall.PtraceSetRegs(c.Pid, &c.regs)  // 更新进程寄存器
}

// ptraceGetRegSet 获取寄存器集
// 包装了系统调用 PTRACE_GETREGS
// 参数：
//   - pid: 目标进程的 ID
//   - regs: 用于存储寄存器值的结构体指针
// 返回：
//   - 如果成功返回 nil，失败返回错误
// 注意：
//   - 进程必须处于被跟踪状态
//   - 通常在系统调用前后使用
func ptraceGetRegSet(pid int, regs *syscall.PtraceRegs) error {
    return syscall.PtraceGetRegs(pid, regs)
}
