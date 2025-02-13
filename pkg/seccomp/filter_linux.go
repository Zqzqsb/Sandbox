// Package seccomp 提供了 seccomp 过滤器的生成功能。
// seccomp (secure computing mode) 是 Linux 内核提供的安全机制，
// 用于限制进程可以使用的系统调用。
package seccomp

import "syscall"

// Filter 是 BPF (Berkeley Packet Filter) 格式的 seccomp 过滤器。
// BPF 是一种在内核空间执行的虚拟机指令集，用于过滤系统调用。
// 每个 SockFilter 结构体表示一条 BPF 指令，包含：
// - Code: 操作码，定义指令的行为（加载、存储、跳转等）
// - Jt/Jf: 条件跳转的目标（true/false）
// - K: 立即数值或内存地址
type Filter []syscall.SockFilter

// SockFprog 将 Filter 转换为内核可以理解的 SockFprog 格式。
// SockFprog 结构体包含：
// - Len: 过滤器程序的长度（指令数量）
// - Filter: 指向过滤器程序第一条指令的指针
//
// 这个方法在调用 prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, prog) 时使用，
// 用于将过滤器安装到内核中。
//
// 注意：Filter 指针必须指向连续的内存区域，因此我们需要获取切片底层数组的指针。
func (f Filter) SockFprog() *syscall.SockFprog {
	// 将 Filter 转换为 SockFilter 切片
	b := []syscall.SockFilter(f)
	
	// 创建 SockFprog 结构体
	return &syscall.SockFprog{
		Len:    uint16(len(b)),     // 过滤器程序的长度
		Filter: &b[0],              // 指向第一条指令的指针
	}
}
