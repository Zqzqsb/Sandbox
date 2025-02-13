package libseccomp

// Action 定义了 seccomp 过滤器的动作类型
// 在内部实现中，Action 是一个 32 位无符号整数：
// - 低 16 位用于基本动作（如 ALLOW、KILL 等）
// - 高 16 位用于附加数据（如错误码、跟踪标志等）
type Action uint32

// Action 定义了对系统调用的处理动作
// 这些常量从 1 开始递增（iota + 1），确保 0 值无效
const (
	ActionAllow Action = iota + 1 // 允许系统调用继续执行
	ActionErrno                   // 返回一个错误码给调用进程
	ActionTrace                   // 通知跟踪器（如 ptrace）并暂停执行
	ActionKill                    // 立即终止进程
)

// MsgDisallow 和 MsgHandle 定义了当进程触发 seccomp 过滤器时
// 需要采取的具体处理方式
//
// 这些常量用于与 tracer（如 ptrace）通信：
// - MsgDisallow：表示应该禁止该系统调用
// - MsgHandle：表示需要由 tracer 处理该系统调用
const (
	MsgDisallow int16 = iota + 1 // 禁止系统调用
	MsgHandle                     // 由 tracer 处理
)

// Action 方法返回基本动作类型（不包含附加数据）
// 通过位掩码 0xffff 提取低 16 位的基本动作值
//
// 例如：
// - 如果 Action = 0x00010001 (ActionAllow | 某些附加数据)
// - 则返回 0x0001 (ActionAllow)
func (a Action) Action() Action {
	return Action(a & 0xffff)
}
