package libseccomp

import (
	"fmt"
	"github.com/elastic/go-seccomp-bpf/arch"
)

// GetInfo 获取当前系统架构的系统调用信息
// arch.GetInfo("") 返回当前系统架构（如 x86_64, arm64 等）的系统调用映射表
// info 包含了系统调用号到系统调用名称的映射关系
var info, errInfo = arch.GetInfo("")

// ToSyscallName 将系统调用号转换为对应的系统调用名称
// 参数：
//   - sysno: 系统调用号（如 x86_64 上 read 的系统调用号是 0）
//
// 返回值：
//   - string: 系统调用名称（如 "read", "write" 等）
//   - error: 如果转换失败则返回错误
//
// 错误情况：
//   - 如果获取系统架构信息失败
//   - 如果系统调用号在当前架构上不存在
func ToSyscallName(sysno uint) (string, error) {
	// 检查是否成功获取到系统架构信息
	if errInfo != nil {
		return "", errInfo
	}

	// 在映射表中查找系统调用号对应的名称
	// info.SyscallNumbers 是一个 map[int]string，
	// 键是系统调用号，值是系统调用名称
	n, ok := info.SyscallNumbers[int(sysno)]
	if !ok {
		return "", fmt.Errorf("syscall no %d does not exits", sysno)
	}

	return n, nil
}
