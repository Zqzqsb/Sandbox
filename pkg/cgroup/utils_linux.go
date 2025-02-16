// Package cgroup 提供了 Linux cgroup 的实现
package cgroup

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// EnsureDirExists 确保目录存在，如果不存在则创建
// path: 目录路径
// 返回：
//   - 如果目录已存在，返回 os.ErrExist
//   - 如果创建失败，返回错误
//   - 如果创建成功，返回 nil
func EnsureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, dirPerm)
	}
	return os.ErrExist
}

// CreateV1ControllerPath 为指定的控制器和前缀创建 cgroup v1 路径
// controller: 控制器名称（如 cpu、memory）
// prefix: 路径前缀
// 返回：完整的控制器路径和可能的错误
func CreateV1ControllerPath(controller, prefix string) (string, error) {
	p := path.Join(basePath, controller, prefix)
	return p, EnsureDirExists(p)
}

const initPath = "init"

// EnableV2Nesting 启用 cgroup v2 的嵌套功能
// 该函数会：
// 1. 将容器中的所有进程迁移到嵌套的 /init 路径下
// 2. 在根 cgroup 中启用所有可用的控制器
// 这是为了支持在容器中使用 cgroup v2 的必要步骤
func EnableV2Nesting() error {
	// 如果不是 v2，直接返回
	if DetectType() != TypeV2 {
		return nil
	}

	// 读取当前 cgroup 中的所有进程
	p, err := readFile(path.Join(basePath, cgroupProcs))
	if err != nil {
		return err
	}
	procs := strings.Split(string(p), "\n")
	if len(procs) == 0 {
		return nil
	}

	// 创建 init 目录
	if err := os.Mkdir(path.Join(basePath, initPath), dirPerm); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	// 将所有进程移动到 init cgroup
	procFile, err := os.OpenFile(path.Join(basePath, initPath, cgroupProcs), os.O_RDWR, filePerm)
	if err != nil {
		return err
	}
	for _, v := range procs {
		if _, err := procFile.WriteString(v); err != nil {
			continue // 忽略单个进程的错误，继续处理其他进程
		}
	}
	procFile.Close()
	return nil
}

// ReadProcesses 读取 cgroup.procs 文件并返回进程 ID 列表
// path: cgroup.procs 文件的路径
// 返回：进程 ID 列表和可能的错误
func ReadProcesses(path string) ([]int, error) {
	content, err := readFile(path)
	if err != nil {
		return nil, err
	}
	procs := strings.Split(string(content), "\n")
	rt := make([]int, len(procs))
	for i, x := range procs {
		if len(x) == 0 {
			continue
		}
		rt[i], err = strconv.Atoi(x)
		if err != nil {
			return nil, err
		}
	}
	return rt, nil
}

// AddProcesses 将进程添加到 cgroup.procs 文件中
// path: cgroup.procs 文件的路径
// procs: 要添加的进程 ID 列表
func AddProcesses(path string, procs []int) error {
	f, err := os.OpenFile(path, os.O_RDWR, filePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range procs {
		if _, err := f.WriteString(strconv.Itoa(p)); err != nil {
			return err
		}
	}
	return nil
}

// DetectType 检测当前系统挂载的 cgroup 类型
// 通过检查 /sys/fs/cgroup 的文件系统类型来判断：
// - 如果是 CGROUP2_SUPER_MAGIC，则为 v2
// - 否则为 v1
func DetectType() Type {
	var st unix.Statfs_t
	if err := unix.Statfs(basePath, &st); err != nil {
		// 忽略错误，默认使用 v1
		return TypeV1
	}
	if st.Type == unix.CGROUP2_SUPER_MAGIC {
		return TypeV2
	}
	return TypeV1
}

// remove 删除指定的文件或目录
// 如果名称为空，则返回 nil
func remove(name string) error {
	if name != "" {
		return os.Remove(name)
	}
	return nil
}

// errPatternHasSeparator 表示模式中包含路径分隔符的错误
var errPatternHasSeparator = errors.New("pattern contains path separator")

// prefixAndSuffix 根据最后一个通配符 "*" 分割模式
// 返回：
// - prefix: "*" 之前的部分
// - suffix: "*" 之后的部分
// - err: 如果模式包含路径分隔符则返回错误
func prefixAndSuffix(pattern string) (prefix, suffix string, err error) {
	// 检查是否包含路径分隔符
	for i := 0; i < len(pattern); i++ {
		if os.IsPathSeparator(pattern[i]) {
			return "", "", errPatternHasSeparator
		}
	}
	// 查找最后一个 "*" 并分割
	if pos := strings.LastIndexByte(pattern, '*'); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	} else {
		prefix = pattern
	}
	return prefix, suffix, nil
}

// readFile 读取文件内容，处理 EINTR 中断
// 在读取 cgroup 文件系统时，可能会因为系统调用中断而需要重试
func readFile(p string) ([]byte, error) {
	data, err := os.ReadFile(p)
	for err != nil && errors.Is(err, syscall.EINTR) {
		data, err = os.ReadFile(p)
	}
	return data, err
}

// writeFile 写入文件内容，处理 EINTR 中断
// 在写入 cgroup 文件系统时，可能会因为系统调用中断而需要重试
func writeFile(p string, content []byte, perm fs.FileMode) error {
	err := os.WriteFile(p, content, perm)
	for err != nil && errors.Is(err, syscall.EINTR) {
		err = os.WriteFile(p, content, perm)
	}
	return err
}

// nextRandom 生成随机数字符串
func nextRandom() string {
	return strconv.Itoa(int(rand.Int32()))
}

// randomBuild 创建带有随机目录名的 cgroup
// 类似于 os.MkdirTemp 的功能
// pattern: 目录名模式，可以包含一个 "*" 作为随机部分
// build: 创建 cgroup 的函数
// 返回：创建的 cgroup 和可能的错误
func randomBuild(pattern string, build func(string) (Cgroup, error)) (Cgroup, error) {
	// 解析模式
	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return nil, fmt.Errorf("cgroup.builder: random %v", err)
	}

	// 尝试创建，最多尝试 10000 次
	try := 0
	for {
		name := prefix + nextRandom() + suffix
		cg, err := build(name)
		if err == nil {
			return cg, nil
		}
		if errors.Is(err, os.ErrExist) || (cg != nil && cg.Existing()) {
			if try++; try < 10000 {
				continue
			}
			return nil, fmt.Errorf("cgroup.builder: tried 10000 times but failed")
		}
		return nil, fmt.Errorf("cgroup.builder: random %v", err)
	}
}
