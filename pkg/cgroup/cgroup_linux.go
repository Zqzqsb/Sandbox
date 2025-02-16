// Package cgroup 提供了对 Linux Control Groups 的管理功能
package cgroup

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

// Cgroup 定义了控制 cgroup 的通用接口
// 该接口同时支持 v1 和 v2 版本的实现
// TODO: 实现 systemd 集成
type Cgroup interface {
	// AddProc 将一个或多个进程添加到 cgroup 中
	// pid 参数可以是一个或多个进程 ID
	AddProc(pid ...int) error

	// Destroy 删除当前 cgroup
	// 注意：删除前会自动将所有进程移动到父 cgroup
	Destroy() error

	// Existing 返回 cgroup 是否已经存在
	// true 表示打开现有 cgroup，false 表示新创建的 cgroup
	Existing() bool

	// Nest 创建一个子 cgroup，并将当前进程移动到新创建的 cgroup 中
	// name 参数指定子 cgroup 的名称
	Nest(name string) (Cgroup, error)

	// CPUUsage 读取 cgroup 的总 CPU 使用量
	// 返回值单位为纳秒
	CPUUsage() (uint64, error)

	// MemoryUsage 读取当前的总内存使用量
	// 返回值单位为字节
	MemoryUsage() (uint64, error)

	// MemoryMaxUsage 读取最大内存使用量
	// 注意：在内核版本低于 5.19 的 cgroup v2 中不存在此功能
	MemoryMaxUsage() (uint64, error)

	// SetCPUBandwidth 设置 CPU 带宽限制
	// quota 和 period 参数单位为纳秒
	// quota/period 表示 CPU 使用率上限
	SetCPUBandwidth(quota, period uint64) error

	// SetCPUSet 设置可用的 CPU 核心
	// 参数格式为 "0-3,5,7" 表示使用 CPU0-3,CPU5,CPU7
	SetCPUSet([]byte) error

	// SetMemoryLimit 设置内存使用上限
	// 参数单位为字节
	SetMemoryLimit(uint64) error

	// SetProcLimit 设置进程数量上限
	// 用于限制 cgroup 中可以创建的进程数
	SetProcLimit(uint64) error

	// Processes 列出 cgroup 中所有进程的 PID
	Processes() ([]int, error)

	// New 基于当前 cgroup 创建一个子 cgroup
	// 参数指定子 cgroup 的名称
	New(string) (Cgroup, error)

	// Random 创建一个随机命名的子 cgroup
	// 参数作为随机名称的前缀
	Random(string) (Cgroup, error)
}

// DetectedCgroupType 定义了系统当前使用的 cgroup 类型
// 在包初始化时通过 DetectType() 函数检测
var DetectedCgroupType = DetectType()

// New 创建一个新的 cgroup
// prefix: cgroup 的名称前缀
// ct: 需要启用的控制器列表
// 如果 cgroup 已存在，则打开现有的 cgroup
func New(prefix string, ct *Controllers) (Cgroup, error) {
	if DetectedCgroupType == TypeV1 {
		return newV1(prefix, ct)
	}
	return newV2(prefix, ct)
}

// loopV1Controllers 遍历 v1 版本的所有控制器
// 对每个启用的控制器执行指定的函数
func loopV1Controllers(ct *Controllers, v1 *V1, f func(string, **v1controller) error) error {
	for _, c := range []struct {
		available bool        // 控制器是否可用
		name      string     // 控制器名称
		cg        **v1controller // 控制器指针
	}{
		{ct.CPU, CPU, &v1.cpu},         // CPU 时间控制器
		{ct.CPUSet, CPUSet, &v1.cpuset}, // CPU 核心绑定控制器
		{ct.CPUAcct, CPUAcct, &v1.cpuacct}, // CPU 统计控制器
		{ct.Memory, Memory, &v1.memory},   // 内存控制器
		{ct.Pids, Pids, &v1.pids},       // 进程数量控制器
	} {
		if !c.available {
			continue
		}
		if err := f(c.name, c.cg); err != nil {
			return err
		}
	}
	return nil
}

// newV1 创建一个新的 v1 版本 cgroup
// prefix: cgroup 名称前缀
// ct: 需要启用的控制器列表
func newV1(prefix string, ct *Controllers) (cg Cgroup, err error) {
	v1 := &V1{
		prefix: prefix,
	}
	// 如果创建失败，清理已创建的目录
	defer func() {
		if err != nil && !v1.existing {
			for _, p := range v1.all {
				remove(p.path)
			}
		}
	}()

	// 为每个控制器创建目录
	if err = loopV1Controllers(ct, v1, func(name string, cg **v1controller) error {
		path, err := CreateV1ControllerPath(name, prefix)
		*cg = newV1Controller(path)
		if errors.Is(err, os.ErrExist) {
			if len(v1.all) == 0 {
				v1.existing = true
			}
			return nil
		}
		if err != nil {
			return err
		}
		v1.all = append(v1.all, *cg)
		return nil
	}); err != nil {
		return
	}

	// 初始化 CPU 核心设置
	// 这是必需的，否则 cpuset 控制器无法正常工作
	if v1.cpuset != nil {
		if err = initCpuset(v1.cpuset.path); err != nil {
			return
		}
	}
	return v1, err
}

// newV2 创建一个新的 v2 版本 cgroup
// prefix: cgroup 名称前缀
// ct: 需要启用的控制器列表
func newV2(prefix string, ct *Controllers) (cg Cgroup, err error) {
	v2 := &V2{
		path:    path.Join(basePath, prefix),
		control: ct,
	}
	// 检查是否已存在
	if _, err := os.Stat(v2.path); err == nil {
		v2.existing = true
	}
	// 如果创建失败，清理已创建的目录
	defer func() {
		if err != nil && !v2.existing {
			remove(v2.path)
		}
	}()

	// 准备控制器启用命令
	s := ct.Names()
	controlMsg := []byte("+" + strings.Join(s, " +"))

	// 从根目录开始，逐级创建目录并启用控制器
	entries := strings.Split(prefix, "/")
	current := ""
	for _, e := range entries {
		parent := current
		current = current + "/" + e
		// 尝试创建目录（如果不存在）
		if _, err := os.Stat(path.Join(basePath, current)); os.IsNotExist(err) {
			if err := os.Mkdir(path.Join(basePath, current), dirPerm); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		// 检查并启用需要的控制器
		ect, err := getAvailableControllerV2(current)
		if err != nil {
			return nil, err
		}
		if ect.Contains(ct) {
			continue
		}
		if err := writeFile(path.Join(basePath, parent, cgroupSubtreeControl), controlMsg, filePerm); err != nil {
			return nil, err
		}
	}
	return v2, nil
}

// OpenExisting 打开一个已存在的 cgroup
// prefix: cgroup 的名称前缀
// ct: 需要的控制器列表
func OpenExisting(prefix string, ct *Controllers) (Cgroup, error) {
	if DetectedCgroupType == TypeV1 {
		return openExistingV1(prefix, ct)
	}
	return openExistingV2(prefix, ct)
}

// openExistingV1 打开一个已存在的 v1 版本 cgroup
func openExistingV1(prefix string, ct *Controllers) (cg Cgroup, err error) {
	v1 := &V1{
		prefix:   prefix,
		existing: true,
	}

	// 遍历并初始化所有控制器
	if err = loopV1Controllers(ct, v1, func(name string, cg **v1controller) error {
		p := path.Join(basePath, name, prefix)
		*cg = newV1Controller(p)
		// 检查目录是否存在
		if _, err := os.Stat(p); err != nil {
			return err
		}
		v1.all = append(v1.all, *cg)
		return nil
	}); err != nil {
		return
	}

	// 初始化 CPU 核心设置
	if v1.cpuset != nil {
		if err = initCpuset(v1.cpuset.path); err != nil {
			return
		}
	}
	return
}

// openExistingV2 打开一个已存在的 v2 版本 cgroup
func openExistingV2(prefix string, ct *Controllers) (cg Cgroup, err error) {
	// 获取可用的控制器
	ect, err := getAvailableControllerV2(prefix)
	if err != nil {
		return nil, err
	}
	// 验证所需的控制器是否都可用
	if !ect.Contains(ct) {
		return nil, fmt.Errorf("openCgroupV2: requesting %v controllers but %v found", ct, ect)
	}
	return &V2{
		path:     path.Join(basePath, prefix),
		control:  ect,
		existing: true,
	}, nil
}
