// Package ptrace 提供了基于 ptrace 的进程跟踪和沙箱运行环境
package ptrace

import (
	"syscall"

	"github.com/zqzqsb/sandbox/pkg/rlimit"
	"github.com/zqzqsb/sandbox/pkg/seccomp"
	"github.com/zqzqsb/sandbox/ptracer"
	"github.com/zqzqsb/sandbox/runner"
)

// Runner 定义了使用 ptrace 安全运行程序的规范
// 它包含了所有必要的配置项，用于控制子进程的运行环境和行为
type Runner struct {
	// Args 定义子进程的命令行参数
	// 格式：[程序名, 参数1, 参数2, ...]
	Args []string

	// Env 定义子进程的环境变量
	// 格式：["KEY=VALUE", ...]
	Env []string

	// WorkDir 定义子进程的工作目录
	// 如果为空，则使用当前目录
	WorkDir string

	// ExecFile 是要执行的文件的文件描述符
	// 用于 fexecve 系统调用，可以在不暴露文件路径的情况下执行程序
	ExecFile uintptr

	// Files 定义了子进程的文件描述符映射
	// 索引对应新进程中的文件描述符编号（从0开始）
	// 例如：Files[0] 对应标准输入，Files[1] 对应标准输出
	Files []uintptr

	// RLimits 定义了通过 setrlimit 设置的资源限制
	// 包括内存限制、CPU 时间限制等
	RLimits []rlimit.RLimit

	// Limit 定义了由跟踪器强制执行的资源限制
	// 这些限制在运行时由 ptrace 强制执行
	Limit runner.Limit

	// Seccomp 定义了安全计算模式过滤器
	// - 文件访问相关的系统调用需要设置为 ActionTrace
	// - 允许的系统调用设置为 ActionAllow
	// - 默认动作应该是 ActionTrace 或 ActionKill
	Seccomp seccomp.Filter

	// Handler 定义了系统调用的处理器
	// 用于处理被跟踪的系统调用
	Handler Handler

	// ShowDetails 控制是否显示详细的调试信息
	// Unsafe 控制是否允许不安全的操作（软禁用而不是杀死进程）
	ShowDetails, Unsafe bool

	// SyncFunc 定义了进程同步函数
	// 主要用于 cgroup 将进程添加到控制组
	// 参数是子进程的 PID
	SyncFunc func(pid int) error
}

// BanRet 定义了系统调用被禁止时的返回值
// 使用 EACCES (Permission denied) 作为默认的错误码
var BanRet = syscall.EACCES

// Handler 定义了文件访问和系统调用的处理接口
// 实现这个接口可以自定义对不同类型访问的处理策略
type Handler interface {
	// CheckRead 检查读取文件的权限
	// 参数是要读取的文件路径
	// 返回跟踪动作：允许、禁止或终止
	CheckRead(string) ptracer.TraceAction

	// CheckWrite 检查写入文件的权限
	// 参数是要写入的文件路径
	// 返回跟踪动作：允许、禁止或终止
	CheckWrite(string) ptracer.TraceAction

	// CheckStat 检查获取文件状态的权限
	// 参数是要检查的文件路径
	// 返回跟踪动作：允许、禁止或终止
	CheckStat(string) ptracer.TraceAction

	// CheckSyscall 检查系统调用的权限
	// 参数是系统调用的名称
	// 返回跟踪动作：允许、禁止或终止
	CheckSyscall(string) ptracer.TraceAction
}
