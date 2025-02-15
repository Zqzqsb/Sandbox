// Package mount 提供了 Linux 系统中挂载点管理的功能
package mount

import (
	"syscall"
)

// Mount 定义了挂载点的基本属性
// 这个结构体用于描述一个挂载操作所需的所有信息
type Mount struct {
	Source string  // 挂载源（如设备文件、目录或特殊文件系统名称）
	Target string  // 挂载目标（挂载点的路径）
	FsType string  // 文件系统类型（如 ext4、tmpfs、proc 等）
	Data   string  // 挂载选项（如 size=64m 等）
	Flags  uintptr // 挂载标志（如 MS_RDONLY、MS_BIND 等）
}

// SyscallParams 定义了执行 mount 系统调用所需的原始参数
// 这个结构体将 Mount 结构体中的字符串转换为 C 风格的字节指针
type SyscallParams struct {
	Source, Target, FsType, Data *byte // C 风格的字符串指针
	Flags                        uintptr // 挂载标志
	Prefixes                     []*byte // 目标路径的所有父目录路径（用于创建挂载点）
	MakeNod                      bool    // 是否需要创建设备节点（用于文件绑定挂载）
}

// ToSyscall 将 Mount 结构体转换为系统调用参数
// 这个方法执行以下操作：
// 1. 将所有字符串转换为 C 风格的字节指针
// 2. 获取目标路径的所有父目录
// 3. 返回可直接用于系统调用的参数集
func (m *Mount) ToSyscall() (*SyscallParams, error) {
	var data *byte
	// 转换源路径
	source, err := syscall.BytePtrFromString(m.Source)
	if err != nil {
		return nil, err
	}
	// 转换目标路径
	target, err := syscall.BytePtrFromString(m.Target)
	if err != nil {
		return nil, err
	}
	// 转换文件系统类型
	fsType, err := syscall.BytePtrFromString(m.FsType)
	if err != nil {
		return nil, err
	}
	// 如果有挂载选项，转换它
	if m.Data != "" {
		data, err = syscall.BytePtrFromString(m.Data)
		if err != nil {
			return nil, err
		}
	}
	// 获取目标路径的所有父目录
	prefix := pathPrefix(m.Target)
	// 将所有路径转换为 C 风格的字符串
	paths, err := arrayPtrFromStrings(prefix)
	if err != nil {
		return nil, err
	}
	return &SyscallParams{
		Source:   source,
		Target:   target,
		FsType:   fsType,
		Flags:    m.Flags,
		Data:     data,
		Prefixes: paths,
	}, nil
}

// pathPrefix 获取路径的所有组成部分
// 例如：对于路径 "/a/b/c"，返回 ["/a", "/a/b", "/a/b/c"]
// 这用于确保在挂载点创建过程中所有必要的父目录都存在
func pathPrefix(path string) []string {
	ret := make([]string, 0)
	// 遍历路径中的每个斜杠
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			ret = append(ret, path[:i]) // 添加到当前斜杠的子路径
		}
	}
	ret = append(ret, path) // 添加完整路径
	return ret
}

// arrayPtrFromStrings 将字符串数组转换为 C 风格的字符串数组
// 这个函数主要用于将路径列表转换为系统调用可用的格式
func arrayPtrFromStrings(str []string) ([]*byte, error) {
	bytes := make([]*byte, 0, len(str))
	for _, s := range str {
		// 将每个字符串转换为 C 风格的字节指针
		b, err := syscall.BytePtrFromString(s)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, b)
	}
	return bytes, nil
}
