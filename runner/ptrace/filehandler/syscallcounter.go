package filehandler
/*
	SyscallCounter 为每个系统调用定义倒计数
	这个计数器在沙箱中的作用是防止程序滥用某些系统调用，比如：
		限制文件打开次数
		限制网络连接次数
		限制创建线程/进程的次数
	通过这种方式，可以更精细地控制程序的行为，提高安全性。
*/
type SyscallCounter map[string]int

// NewSyscallCounter 创建新的 SyscallCounter
func NewSyscallCounter() SyscallCounter {
	return SyscallCounter(make(map[string]int))
}

// Add 向 SyscallCounter 添加单个计数器
func (s SyscallCounter) Add(name string, count int) {
	s[name] = count
}

// AddRange 向 SyscallCounter 添加多个计数器
func (s SyscallCounter) AddRange(m map[string]int) {
	for k, v := range m {
		s[k] = v
	}
}

// Check 返回 inside, allow
// inside: 表示系统调用是否在计数器中
// allow: 表示是否允许继续执行
func (s SyscallCounter) Check(syscallName string) (bool, bool) {
	n, o := s[syscallName]
	if o {
		s[syscallName] = n - 1
		if n <= 1 {
			return true, false
		}
		return true, true
	}
	return false, true
}
