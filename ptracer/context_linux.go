package ptracer

import (
	"os"
	"syscall"
)

// Context 是当前系统调用陷阱的上下文
// 用于获取系统调用号和参数
type Context struct {
	// Pid 是当前上下文进程的 pid
	Pid int
	// 当前寄存器上下文（平台相关）
	regs syscall.PtraceRegs
}

var (
	// UseVMReadv 决定是否使用 ProcessVMReadv 系统调用来读取字符串
	// 初始为 true，如果尝试失败并返回 ENOSYS 则变为 false
	UseVMReadv = true
	pageSize   = 4 << 10
)

func init() {
	pageSize = os.Getpagesize()
}


/*
	使用示例:
	ctx, err := getTrapContext(1234)  // 获取 PID 1234 的进程上下文
	if err != nil {
		处理错误
	}
	ctx.Pid == 1234
	ctx.regs 包含进程寄存器状态
*/
func getTrapContext(pid int) (*Context, error) {
	var regs syscall.PtraceRegs
	err := ptraceGetRegSet(pid, &regs)
	if err != nil {
		return nil, err
	}
	return &Context{
		Pid:  pid,
		regs: regs,
	}, nil
}

// GetString 从进程数据段获取字符串
// 参数：
//   - addr: 目标进程内存中的字符串地址
// 返回：
//   - 读取到的字符串，如果读取失败则返回空字符串
// 注意：
//   - 首先尝试使用更高效的 ProcessVMReadv
//   - 如果系统不支持，则回退到 ptrace 读取
//   - 字符串以 null 字节(\0)结尾
func (c *Context) GetString(addr uintptr) string {
    // 创建缓冲区，大小为系统最大路径长度
    buff := make([]byte, syscall.PathMax)

    // 尝试使用 ProcessVMReadv（更高效的读取方式）
    if UseVMReadv {
        if err := vmReadStr(c.Pid, addr, buff); err != nil {
            // 如果系统不支持 ProcessVMReadv（返回 ENOSYS）
            // 则禁用此功能，后续使用 ptrace 读取
            if no, ok := err.(syscall.Errno); ok {
                if no == syscall.ENOSYS {
                    UseVMReadv = false
                }
            }
        } else {
            // ProcessVMReadv 成功，返回读取的字符串
            return string(buff)
        }
    }

    // 如果 ProcessVMReadv 不可用或失败
    // 使用 ptrace 读取字符串
    if err := ptraceReadStr(c.Pid, addr, buff); err != nil {
        return ""  // 读取失败返回空字符串
    }
    return string(buff)  // 返回成功读取的字符串
}
