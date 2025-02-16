// Package ptrace 实现了基于 ptrace 的进程跟踪和沙箱控制
package ptrace

import (
	"context"
	"os"

	"github.com/zqzqsb/sandbox/pkg/forkexec"
	"github.com/zqzqsb/sandbox/ptracer"
	"github.com/zqzqsb/sandbox/runner"
)

// Run 启动进程跟踪
// 该函数实现了以下功能：
// 1. 设置子进程运行环境（包括参数、环境变量、文件描述符等）
// 2. 配置 ptrace 和 seccomp 过滤器
// 3. 创建跟踪器并开始跟踪进程
//
// 参数：
// - c: 上下文，用于控制跟踪过程的生命周期
//
// 返回：
// - runner.Result: 包含进程运行结果的信息
func (r *Runner) Run(c context.Context) runner.Result {
	// 创建子进程运行器，配置运行环境
	ch := &forkexec.Runner{
		Args:     r.Args,     // 命令行参数
		Env:      r.Env,      // 环境变量
		ExecFile: r.ExecFile, // 可执行文件路径
		RLimits:  r.RLimits,  // 资源限制
		Files:    r.Files,    // 文件描述符
		WorkDir:  r.WorkDir,  // 工作目录
		Seccomp:  r.Seccomp.SockFprog(), // seccomp 过滤器
		Ptrace:   true,       // 启用 ptrace 跟踪
		SyncFunc: r.SyncFunc, // 同步函数

		// 如果是 root 用户，在同步后取消 cgroup 共享
		// 这样可以确保子进程在自己的 cgroup 中运行
		UnshareCgroupAfterSync: os.Getuid() == 0,
	}

	// 创建跟踪处理器
	th := &tracerHandler{
		ShowDetails: r.ShowDetails, // 是否显示详细信息
		Unsafe:      r.Unsafe,      // 是否允许不安全操作
		Handler:     r.Handler,     // 系统调用处理器
	}

	// 创建进程跟踪器
	tracer := ptracer.Tracer{
		Handler: th,      // 跟踪处理器
		Runner:  ch,      // 子进程运行器
		Limit:   r.Limit, // 资源限制
	}

	// 开始跟踪并返回结果
	return tracer.Trace(c)
}
