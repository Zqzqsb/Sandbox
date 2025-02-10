package runner

// Status 是结果状态
type Status int

// 程序运行器的结果状态
const (
	StatusInvalid Status = iota // 0 未初始化
	// 正常
	StatusNormal // 1 正常

	// 资源限制超出
	StatusTimeLimitExceeded   // 2 时间限制超出
	StatusMemoryLimitExceeded // 3 内存限制超出
	StatusOutputLimitExceeded // 4 输出限制超出

	// 未授权访问
	StatusDisallowedSyscall // 5 禁止的系统调用

	// 运行时错误
	StatusSignalled         // 6 被信号终止
	StatusNonzeroExitStatus // 7 非零退出状态

	// 程序运行器错误
	StatusRunnerError // 8 运行器错误
)

var (
	statusString = []string{
		"无效",
		"",
		"超出时间限制",
		"超出内存限制",
		"超出输出限制",
		"禁止的系统调用",
		"被信号终止",
		"非零退出状态",
		"运行器错误",
	}
)

func (t Status) String() string {
	i := int(t)
	if i >= 0 && i < len(statusString) {
		return statusString[i]
	}
	return statusString[0]
}

func (t Status) Error() string {
	return t.String()
}
