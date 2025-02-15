package forkexec

import (
	"syscall"
)

// prepareExec 准备 execve 系统调用所需的参数
// Args: 要执行的命令及其参数数组，Args[0] 是命令本身
// Env: 环境变量数组
// 返回值:
// - *byte: 命令路径（argv0）
// - []*byte: 命令参数数组（argv）
// - []*byte: 环境变量数组（env）
// - error: 错误信息
func prepareExec(Args, Env []string) (*byte, []*byte, []*byte, error) {
	// 将命令路径（Args[0]）转换为 C 风格的字符串（以 null 结尾的字节数组）
	argv0, err := syscall.BytePtrFromString(Args[0])
	if err != nil {
		return nil, nil, nil, err
	}
	// 将所有命令参数转换为 C 风格的字符串数组
	// 这是因为 execve 系统调用需要 C 风格的参数
	argv, err := syscall.SlicePtrFromStrings(Args)
	if err != nil {
		return nil, nil, nil, err
	}
	// 将环境变量数组转换为 C 风格的字符串数组
	// 每个环境变量的格式为 "KEY=VALUE"
	env, err := syscall.SlicePtrFromStrings(Env)
	if err != nil {
		return nil, nil, nil, err
	}
	return argv0, argv, env, nil
}

// prepareFds 准备文件描述符数组
// files: 文件描述符的 uintptr 数组
// 返回值:
// - []int: 转换后的文件描述符数组
// - int: 下一个可用的文件描述符编号
func prepareFds(files []uintptr) ([]int, int) {
	// 创建整型文件描述符数组
	fd := make([]int, len(files))
	// nextfd 将是最大文件描述符值加1
	// 这确保了新创建的文件描述符不会与现有的冲突
	nextfd := len(files)
	for i, ufd := range files {
		// 找到最大的文件描述符值
		if nextfd < int(ufd) {
			nextfd = int(ufd)
		}
		// 将 uintptr 转换为 int
		fd[i] = int(ufd)
	}
	// 返回转换后的数组和下一个可用的文件描述符编号
	nextfd++
	return fd, nextfd
}

// syscallStringFromString 将 Go 字符串转换为系统调用所需的 *byte 格式
// str: 输入字符串
// 返回值:
// - *byte: 如果输入非空，返回 C 风格字符串；如果输入为空，返回 nil
// - error: 转换过程中的错误
// 注意：该函数主要用于处理可选的字符串参数，如主机名、工作目录等
func syscallStringFromString(str string) (*byte, error) {
	if str != "" {
		return syscall.BytePtrFromString(str)
	}
	return nil, nil
}
