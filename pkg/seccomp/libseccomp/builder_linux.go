package libseccomp

import (
	"syscall"

	"github.com/zqzqsb/sandbox/pkg/seccomp"
	libseccomp "github.com/elastic/go-seccomp-bpf"
	"golang.org/x/net/bpf"
)

// Builder 用于构建 seccomp 过滤器
// 采用 Builder 模式，提供简单的接口来创建复杂的过滤规则
type Builder struct {
	Allow []string  // 允许执行的系统调用列表
	Trace []string  // 需要跟踪的系统调用列表
	Default Action  // 默认动作（当系统调用不在上述列表中时）
}

// actTrace 定义了跟踪动作的全局变量
// 使用全局变量可以避免重复创建相同的动作对象
var actTrace = libseccomp.ActionTrace

// Build 构建过滤器
// 将 Builder 中的配置转换为可执行的 BPF 过滤器
//
// 过程：
// 1. 创建过滤策略
// 2. 编译为 BPF 程序
// 3. 转换为内核可读格式
func (b *Builder) Build() (seccomp.Filter, error) {
	// 创建 libseccomp 策略
	policy := libseccomp.Policy{
		DefaultAction: ToSeccompAction(b.Default),  // 设置默认动作
		Syscalls: []libseccomp.SyscallGroup{
			{
				Action: libseccomp.ActionAllow,  // 允许执行的系统调用
				Names:  b.Allow,
			},
			{
				Action: actTrace,  // 需要跟踪的系统调用
				Names:  b.Trace,
			},
		},
	}

	// 将策略编译为 BPF 程序
	program, err := policy.Assemble()
	if err != nil {
		return nil, err
	}

	// 转换为内核可读的格式
	return ExportBPF(program)
}

// ExportBPF 将 libseccomp 过滤器转换为内核可读的 BPF 内容
//
// 参数：
//   - filter: BPF 指令序列
//
// 返回：
//   - seccomp.Filter: 转换后的过滤器
//   - error: 转换过程中的错误
func ExportBPF(filter []bpf.Instruction) (seccomp.Filter, error) {
	// 将 BPF 指令汇编为原始指令
	raw, err := bpf.Assemble(filter)
	if err != nil {
		return nil, err
	}
	// 转换为 SockFilter 格式
	return sockFilter(raw), nil
}

// sockFilter 将原始 BPF 指令转换为内核使用的 SockFilter 格式
//
// 参数：
//   - raw: 原始 BPF 指令序列
//
// 返回：
//   - []syscall.SockFilter: 转换后的过滤器指令序列
//
// 转换过程：
// - Code: 操作码
// - Jt/Jf: 跳转目标
// - K: 立即数/地址
func sockFilter(raw []bpf.RawInstruction) []syscall.SockFilter {
	filter := make([]syscall.SockFilter, 0, len(raw))
	for _, instruction := range raw {
		filter = append(filter, syscall.SockFilter{
			Code: instruction.Op,   // 操作码
			Jt:   instruction.Jt,   // 真跳转目标
			Jf:   instruction.Jf,   // 假跳转目标
			K:    instruction.K,    // 立即数/地址
		})
	}
	return filter
}
