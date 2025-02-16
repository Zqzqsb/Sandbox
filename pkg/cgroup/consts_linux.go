// Package cgroup 提供了对 Linux Control Groups (cgroups) 的管理功能
package cgroup

// Cgroup 相关的常量定义
const (
	// basePath 是 cgroup 的根目录路径
	// 在 Linux 系统中，cgroup 文件系统通常挂载在 /sys/fs/cgroup 目录下
	basePath = "/sys/fs/cgroup"

	// cgroupProcs 是存储 cgroup 中进程 ID 的文件名
	// 每个 cgroup 目录下都有这个文件，用于管理属于该 cgroup 的进程
	cgroupProcs = "cgroup.procs"

	// procCgroupsPath 是系统支持的所有 cgroup 子系统的信息文件
	// 包含了子系统名称、层级 ID、已挂载数量等信息
	procCgroupsPath = "/proc/cgroups"

	// procSelfCgroup 是当前进程的 cgroup 信息文件
	// 记录了进程所属的 cgroup 路径
	procSelfCgroup = "/proc/self/cgroup"

	// cgroupSubtreeControl 用于控制子树中可用的控制器
	// 通过写入 "+controller" 或 "-controller" 来启用或禁用控制器
	cgroupSubtreeControl = "cgroup.subtree_control"

	// cgroupControllers 列出了当前 cgroup 中可用的所有控制器
	// 只读文件，显示可以被启用的控制器列表
	cgroupControllers = "cgroup.controllers"

	// filePerm 定义了创建文件时的默认权限
	// 0644 表示: 所有者可读写，组用户和其他用户只读
	filePerm = 0644

	// dirPerm 定义了创建目录时的默认权限
	// 0755 表示: 所有者可读写执行，组用户和其他用户可读和执行
	dirPerm = 0755

	// CPU 控制器名称，用于 CPU 时间和带宽限制
	CPU = "cpu"

	// CPUAcct 控制器名称，用于 CPU 资源使用统计
	CPUAcct = "cpuacct"

	// CPUSet 控制器名称，用于 CPU 核心绑定
	CPUSet = "cpuset"

	// Memory 控制器名称，用于内存使用限制和统计
	Memory = "memory"

	// Pids 控制器名称，用于限制进程数量
	Pids = "pids"
)

// Type 定义了 cgroup 的版本类型
// cgroup 有两个主要版本：v1 和 v2，它们的实现和使用方式有所不同
type Type int

// cgroup 版本的枚举值
const (
	// TypeV1 表示 cgroup v1
	// v1 版本中每个子系统都是独立的层级结构
	TypeV1 = iota + 1

	// TypeV2 表示 cgroup v2
	// v2 版本使用统一的层级结构，所有控制器在同一个层级中
	TypeV2
)

// String 返回 Type 的字符串表示
// 用于日志记录和错误信息展示
func (t Type) String() string {
	switch t {
	case TypeV1:
		return "v1"
	case TypeV2:
		return "v2"
	default:
		return "invalid"
	}
}
