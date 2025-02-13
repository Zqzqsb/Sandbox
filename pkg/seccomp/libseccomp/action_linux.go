package libseccomp

import (
	libseccomp "github.com/elastic/go-seccomp-bpf"
)

// ToSeccompAction 将我们的 Action 类型转换为 libseccomp 库支持的动作类型
//
// 参数：
//   - a: 我们定义的 Action 类型
//
// 返回值：
//   - libseccomp.Action: libseccomp 库使用的动作类型
//
// 转换对应关系：
//   - ActionAllow -> libseccomp.ActionAllow    (允许系统调用)
//   - ActionErrno -> libseccomp.ActionErrno    (返回错误)
//   - ActionTrace -> libseccomp.ActionTrace    (跟踪系统调用)
//   - 其他       -> libseccomp.ActionKillProcess (终止进程)
func ToSeccompAction(a Action) libseccomp.Action {
	// 提取基本动作（不包含附加数据）
	var action libseccomp.Action
	switch a.Action() {
	case ActionAllow:
		action = libseccomp.ActionAllow   // 允许系统调用继续执行
	case ActionErrno:
		action = libseccomp.ActionErrno   // 返回错误给调用进程
	case ActionTrace:
		action = libseccomp.ActionTrace   // 通知 tracer 并暂停执行
	default:
		action = libseccomp.ActionKillProcess  // 默认情况：终止进程
	}

	// 注意：SECCOMP_RET_DATA 存储在返回值的低 16 位
	// 这部分功能目前在 go-seccomp-bpf 库中并未正式支持
	// 如果需要设置返回数据，可以使用以下代码：
	// action = action.WithReturnData(int(a.ReturnCode()))
	
	return action
}
