//go:build linux
// +build linux

package ptracer

import "github.com/zqzqsb/sandbox/runner"

// TraceAction 定义了 TraceHandle 返回的动作
type TraceAction int

const (
	// TraceAllow 不做任何操作
	TraceAllow TraceAction = iota
	// TraceBan 跳过系统调用并设置由 SetReturnCode 指定的返回代码
	TraceBan
	// TraceKill 表示检测到危险操作
	TraceKill
)

// Tracer 定义了一个 ptracer 实例
type Tracer struct {
	Handler
	Runner
	runner.Limit
}

// Runner 表示进程运行器
type Runner interface {
	// Start 启动子进程并返回 pid 和错误（如果失败）
	// 子进程应该启用 ptrace 并在 ptrace 之前停止
	Start() (int, error)
}

// Handler 定义了跟踪系统调用的自定义处理器
type Handler interface {
	// Handle 返回对被跟踪程序采取的动作
	Handle(*Context) TraceAction

	// Debug 在调试模式下打印调试信息
	Debug(v ...interface{})
}
