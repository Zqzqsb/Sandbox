// Package filehandler 提供了文件访问控制和系统调用监控的功能
package filehandler

import (
	"github.com/zqzqsb/sandbox/ptracer"
)

// Handler 定义了文件访问限制处理器，用于调用 ptrace 安全运行器
// 它包含两个主要组件：
// 1. FileSet: 管理文件访问权限集合
// 2. SyscallCounter: 跟踪和限制系统调用
type Handler struct {
	FileSet        *FileSets       // 文件权限集合，包含可读/可写/可查看状态/软禁止的文件列表
	SyscallCounter SyscallCounter  // 系统调用计数器，用于限制系统调用的次数
}

// CheckRead 检查文件是否有读取权限
// 参数：
//   - fn: 要检查的文件路径
// 返回：
//   - ptracer.TraceAction: 
//     * TraceAllow: 允许读取
//     * TraceBan: 软禁止（跳过操作但继续运行）
//     * TraceKill: 终止程序
func (h *Handler) CheckRead(fn string) ptracer.TraceAction {
	if !h.FileSet.IsReadableFile(fn) {
		return h.onDgsFileDetect(fn)  // 如果文件不可读，交给违规处理函数处理
	}
	return ptracer.TraceAllow  // 允许读取
}

// CheckWrite 检查文件是否有写入权限
// 参数：
//   - fn: 要检查的文件路径
// 返回：
//   - ptracer.TraceAction: 同 CheckRead
func (h *Handler) CheckWrite(fn string) ptracer.TraceAction {
	if !h.FileSet.IsWritableFile(fn) {
		return h.onDgsFileDetect(fn)  // 如果文件不可写，交给违规处理函数处理
	}
	return ptracer.TraceAllow  // 允许写入
}

// CheckStat 检查文件是否有状态查看权限（如 stat, lstat 等系统调用）
// 参数：
//   - fn: 要检查的文件路径
// 返回：
//   - ptracer.TraceAction: 同 CheckRead
func (h *Handler) CheckStat(fn string) ptracer.TraceAction {
	if !h.FileSet.IsStatableFile(fn) {
		return h.onDgsFileDetect(fn)  // 如果文件状态不可查看，交给违规处理函数处理
	}
	return ptracer.TraceAllow  // 允许查看状态
}

// CheckSyscall 检查系统调用是否允许执行
// 这个函数处理除文件操作之外的其他系统调用
// 参数：
//   - syscallName: 系统调用的名称
// 返回：
//   - ptracer.TraceAction:
//     * TraceAllow: 允许系统调用
//     * TraceBan: 软禁止（跳过系统调用）
//     * TraceKill: 终止程序

func (h *Handler) CheckSyscall(syscallName string) ptracer.TraceAction {
	// 检查系统调用是否在跟踪列表中，并且是否超过限制
	if inside, allow := h.SyscallCounter.Check(syscallName); inside {
		if allow {
			return ptracer.TraceAllow  // 在限制范围内，允许执行
		}
		return ptracer.TraceKill      // 超过限制，终止程序
	}
	return ptracer.TraceBan          // 不在跟踪列表中，软禁止
}

// onDgsFileDetect 处理文件访问违规情况
// 根据文件是否在软禁止列表中决定采取的行动
// 参数：
//   - fn: 违规访问的文件路径
// 返回：
//   - ptracer.TraceAction:
//     * TraceBan: 文件在软禁止列表中，跳过操作
//     * TraceKill: 文件不在软禁止列表中，终止程序
func (h *Handler) onDgsFileDetect(fn string) ptracer.TraceAction {
	if h.FileSet.IsSoftBanFile(fn) {
		return ptracer.TraceBan    // 软禁止：跳过操作但允许程序继续运行
	}
	return ptracer.TraceKill      // 硬禁止：立即终止程序
}
