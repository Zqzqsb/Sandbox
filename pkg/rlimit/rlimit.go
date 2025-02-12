// Package rlimit 提供了通过 setrlimit 系统调用设置 Linux 资源限制的数据结构
package rlimit

import (
	"fmt"
	"strings"
	"syscall"
)

// RLimits 定义了通过 setrlimit 系统调用应用到被追踪进程的资源限制
type RLimits struct {
	CPU          uint64 // CPU 时间限制（秒）
	CPUHard      uint64 // 硬性 CPU 时间限制（秒）
	Data         uint64 // 数据段大小限制（字节）
	FileSize     uint64 // 文件大小限制（字节）
	Stack        uint64 // 栈大小限制（字节）
	AddressSpace uint64 // 地址空间限制（字节）
	OpenFile     uint64 // 打开文件数量限制
	DisableCore  bool   // 是否禁用 core dump
}

// RLimit 是 Linux setrlimit 定义的资源限制
type RLimit struct {
	// Res 是资源类型（例如 syscall.RLIMIT_CPU）
	Res int
	// Rlim 是应用到该资源的限制
	Rlim syscall.Rlimit
}

// getRlimit 创建一个新的 Rlimit 结构体
func getRlimit(cur, max uint64) syscall.Rlimit {
	return syscall.Rlimit{Cur: cur, Max: max}
}

// PrepareRLimit 为被追踪进程创建 rlimit 结构体
// TimeLimit 单位为秒，SizeLimit 单位为字节
func (r *RLimits) PrepareRLimit() []RLimit {
	var ret []RLimit

	// CPU 时间限制
	if r.CPU > 0 {
		cpuHard := r.CPUHard
		if cpuHard < r.CPU {
			cpuHard = r.CPU
		}
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_CPU,
			Rlim: getRlimit(r.CPU, cpuHard),
		})
	}

	// 数据段大小限制
	if r.Data > 0 {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_DATA,
			Rlim: getRlimit(r.Data, r.Data),
		})
	}

	// 文件大小限制
	if r.FileSize > 0 {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_FSIZE,
			Rlim: getRlimit(r.FileSize, r.FileSize),
		})
	}

	// 栈大小限制
	if r.Stack > 0 {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_STACK,
			Rlim: getRlimit(r.Stack, r.Stack),
		})
	}

	// 地址空间限制
	if r.AddressSpace > 0 {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_AS,
			Rlim: getRlimit(r.AddressSpace, r.AddressSpace),
		})
	}

	// 打开文件数量限制
	if r.OpenFile > 0 {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_NOFILE,
			Rlim: getRlimit(r.OpenFile, r.OpenFile),
		})
	}

	// 禁用 core dump
	if r.DisableCore {
		ret = append(ret, RLimit{
			Res:  syscall.RLIMIT_CORE,
			Rlim: getRlimit(0, 0),
		})
	}

	return ret
}

// String 返回 RLimit 的字符串表示
func (r RLimit) String() string {
	var t string
	switch r.Res {
	case syscall.RLIMIT_CPU:
		return fmt.Sprintf("CPU[%d s:%d s]", r.Rlim.Cur, r.Rlim.Max)
	case syscall.RLIMIT_NOFILE:
		return fmt.Sprintf("OpenFile[%d:%d]", r.Rlim.Cur, r.Rlim.Max)
	case syscall.RLIMIT_DATA:
		t = "Data"
	case syscall.RLIMIT_FSIZE:
		t = "File"
	case syscall.RLIMIT_STACK:
		t = "Stack"
	case syscall.RLIMIT_AS:
		t = "AddressSpace"
	case syscall.RLIMIT_CORE:
		t = "Core"
	default:
		t = fmt.Sprintf("Resource(%d)", r.Res)
	}
	return fmt.Sprintf("%s[%d]", t, r.Rlim.Cur)
}

// String 返回 RLimits 的字符串表示
func (r *RLimits) String() string {
	var s []string
	if r.CPU > 0 {
		s = append(s, fmt.Sprintf("CPU=%d", r.CPU))
	}
	if r.CPUHard > 0 {
		s = append(s, fmt.Sprintf("CPUHard=%d", r.CPUHard))
	}
	if r.Data > 0 {
		s = append(s, fmt.Sprintf("Data=%d", r.Data))
	}
	if r.FileSize > 0 {
		s = append(s, fmt.Sprintf("FileSize=%d", r.FileSize))
	}
	if r.Stack > 0 {
		s = append(s, fmt.Sprintf("Stack=%d", r.Stack))
	}
	if r.AddressSpace > 0 {
		s = append(s, fmt.Sprintf("AddressSpace=%d", r.AddressSpace))
	}
	if r.OpenFile > 0 {
		s = append(s, fmt.Sprintf("OpenFile=%d", r.OpenFile))
	}
	if r.DisableCore {
		s = append(s, "DisableCore=true")
	}
	return fmt.Sprintf("RLimits{%s}", strings.Join(s, ", "))
}
