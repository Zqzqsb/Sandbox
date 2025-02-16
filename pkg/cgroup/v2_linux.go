// Package cgroup 提供了 Linux cgroups v2 的实现
package cgroup

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

// V2 提供了 cgroup v2 的接口实现
// cgroup v2 相比 v1 使用统一的层级结构，所有控制器都挂载在同一个层级下
type V2 struct {
	path        string      // cgroup 在文件系统中的路径
	control     *Controllers // 可用的控制器（如 cpu、memory 等）
	subtreeOnce sync.Once   // 确保 subtree 控制只初始化一次
	subtreeErr  error       // 存储 subtree 初始化时的错误
	existing    bool        // 标记是否是已存在的 cgroup
}

// 确保 V2 实现了 Cgroup 接口
var _ Cgroup = &V2{}

// String 返回 cgroup 的字符串表示，包含路径和可用控制器
func (c *V2) String() string {
	ct, _ := getAvailableControllerV2path(path.Join(c.path, cgroupControllers))
	return "v2(" + c.path + ")" + ct.String()
}

// AddProc 将指定的进程添加到这个 cgroup 中
// pids: 要添加的进程 ID 列表
func (c *V2) AddProc(pids ...int) error {
	return AddProcesses(path.Join(c.path, cgroupProcs), pids)
}

// Processes 返回该 cgroup 中的所有进程 ID
func (c *V2) Processes() ([]int, error) {
	return ReadProcesses(path.Join(c.path, cgroupProcs))
}

// New 基于当前 cgroup 创建一个新的子 cgroup
// name: 新 cgroup 的名称
func (c *V2) New(name string) (Cgroup, error) {
	// 确保启用了子树控制
	if err := c.enableSubtreeControl(); err != nil {
		return nil, err
	}
	// 创建新的 cgroup 实例
	v2 := &V2{
		path:    path.Join(c.path, name),
		control: c.control,
	}
	// 创建目录
	if err := os.Mkdir(v2.path, dirPerm); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		v2.existing = true
	}
	return v2, nil
}

// Nest 创建一个子 cgroup 并将当前进程移动到新创建的 cgroup 中
// name: 新 cgroup 的名称
func (c *V2) Nest(name string) (Cgroup, error) {
	// 创建新的 cgroup 实例
	v2 := &V2{
		path:    path.Join(c.path, name),
		control: c.control,
	}
	// 创建目录
	if err := os.Mkdir(v2.path, dirPerm); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		v2.existing = true
	}
	// 获取当前 cgroup 中的所有进程
	p, err := c.Processes()
	if err != nil {
		return nil, err
	}
	// 将进程移动到新的 cgroup
	if err := v2.AddProc(p...); err != nil {
		return nil, err
	}
	// 启用子树控制
	if err := c.enableSubtreeControl(); err != nil {
		return nil, err
	}
	return v2, nil
}

// enableSubtreeControl 启用子树控制器
// 这允许在子 cgroup 中使用控制器
func (c *V2) enableSubtreeControl() error {
	c.subtreeOnce.Do(func() {
		// 获取可用的控制器
		ct, err := getAvailableControllerV2path(path.Join(c.path, cgroupControllers))
		if err != nil {
			c.subtreeErr = err
			return
		}
		// 获取已启用的子树控制器
		ect, err := getAvailableControllerV2path(path.Join(c.path, cgroupSubtreeControl))
		if err != nil {
			c.subtreeErr = err
			return
		}
		// 如果所有需要的控制器都已启用，直接返回
		if ect.Contains(ct) {
			return
		}
		// 启用所需的控制器
		s := ct.Names()
		controlMsg := []byte("+" + strings.Join(s, " +"))
		c.subtreeErr = writeFile(path.Join(c.path, cgroupSubtreeControl), controlMsg, filePerm)
	})
	return c.subtreeErr
}

// Random 创建一个随机命名的子 cgroup
// pattern: 名称生成模式
func (c *V2) Random(pattern string) (Cgroup, error) {
	return randomBuild(pattern, c.New)
}

// Destroy 销毁这个 cgroup
// 如果是已存在的 cgroup（不是通过 New 创建的），则不会被删除
func (c *V2) Destroy() error {
	if !c.existing {
		return remove(c.path)
	}
	return nil
}

// Existing 返回这个 cgroup 是否是已存在的（而不是新创建的）
func (c *V2) Existing() bool {
	return c.existing
}

// CPUUsage 读取 CPU 使用统计信息（以纳秒为单位）
func (c *V2) CPUUsage() (uint64, error) {
	b, err := c.ReadFile("cpu.stat")
	if err != nil {
		return 0, err
	}
	// 解析 cpu.stat 文件内容
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) == 2 && parts[0] == "usage_usec" {
			v, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, err
			}
			return uint64(v) * 1000, nil // 转换为纳秒
		}
	}
	return 0, os.ErrNotExist
}

// MemoryUsage 读取当前内存使用量
func (c *V2) MemoryUsage() (uint64, error) {
	if !c.control.Memory {
		return 0, ErrNotInitialized
	}
	return c.ReadUint("memory.current")
}

// MemoryMaxUsage 读取峰值内存使用量
func (c *V2) MemoryMaxUsage() (uint64, error) {
	if !c.control.Memory {
		return 0, ErrNotInitialized
	}
	return c.ReadUint("memory.peak")
}

// SetCPUBandwidth 设置 CPU 带宽限制
// quota: CPU 配额（微秒）
// period: 周期（微秒）
func (c *V2) SetCPUBandwidth(quota, period uint64) error {
	if !c.control.CPU {
		return ErrNotInitialized
	}
	content := strconv.FormatUint(quota, 10) + " " + strconv.FormatUint(period, 10)
	return c.WriteFile("cpu.max", []byte(content))
}

// SetCPUSet 设置 CPU 核心亲和性
func (c *V2) SetCPUSet(content []byte) error {
	if !c.control.CPUSet {
		return ErrNotInitialized
	}
	return c.WriteFile("cpuset.cpus", content)
}

// SetMemoryLimit 设置内存使用上限
func (c *V2) SetMemoryLimit(l uint64) error {
	if !c.control.Memory {
		return ErrNotInitialized
	}
	return c.WriteUint("memory.max", l)
}

// SetProcLimit 设置进程数量上限
func (c *V2) SetProcLimit(l uint64) error {
	if !c.control.Pids {
		return ErrNotInitialized
	}
	return c.WriteUint("pids.max", l)
}

// WriteUint 将 uint64 类型的值写入指定文件
func (c *V2) WriteUint(filename string, i uint64) error {
	return c.WriteFile(filename, []byte(strconv.FormatUint(i, 10)))
}

// ReadUint 从指定文件读取 uint64 类型的值
func (c *V2) ReadUint(filename string) (uint64, error) {
	b, err := c.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	s, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, err
	}
	return s, nil
}

// WriteFile 写入 cgroup 文件内容
// 处理写入慢速设备（cgroup）时可能出现的 EINTR 错误
func (c *V2) WriteFile(name string, content []byte) error {
	p := path.Join(c.path, name)
	return writeFile(p, content, filePerm)
}

// ReadFile 读取 cgroup 文件内容
// 处理读取慢速设备（cgroup）时可能出现的 EINTR 错误
func (c *V2) ReadFile(name string) ([]byte, error) {
	p := path.Join(c.path, name)
	return readFile(p)
}
