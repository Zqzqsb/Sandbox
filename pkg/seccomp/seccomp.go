package seccomp

// Action 定义了系统调用的处理动作
type Action uint32

// Action 常量定义
const (
	ActionInvalid Action = iota // 无效动作
	ActionAllow                 // 允许系统调用
	ActionErrno                 // 返回错误码
	ActionTrace                 // 追踪系统调用
	ActionKill                  // 终止进程
)

// ReturnCode 获取动作的返回码
func (a Action) ReturnCode() uint16 {
	return uint16(a >> 16)
}

// WithReturnCode 设置动作的返回码
func (a Action) WithReturnCode(code uint16) Action {
	return a | Action(code)<<16
}

// Action 获取基本动作（不包含返回码）
func (a Action) Action() Action {
	return Action(a & 0xffff)
}
