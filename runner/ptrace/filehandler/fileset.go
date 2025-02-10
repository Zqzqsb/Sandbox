package filehandler

import (
	"path/filepath"
)

/*
这个组件在沙箱中的作用是：
	安全隔离：限制程序只能访问允许的文件和目录
	灵活控制：可以精细地控制文件的读/写/查看权限
	分层管理：通过目录层级结构简化权限管理
	防止绕过：考虑了符号链接等特殊情况，防止权限检查被绕过
	这是沙箱安全性的重要保障，确保运行的程序只能在允许的范围内访问文件系统
*/

// FileSet 在分层集合中存储文件权限
type FileSet struct {
	Set        map[string]bool // 存储文件路径和权限标记
	SystemRoot bool            // 是否允许访问根目录
}

// FilePerm 存储应用于文件的权限
type FilePerm int

// FilePermWrite / Read / Stat 是权限常量
const (
	FilePermWrite = iota + 1
	FilePermRead
	FilePermStat
)

// NewFileSet 创建新的文件集
func NewFileSet() FileSet {
	return FileSet{make(map[string]bool), false}
}

/*
	fs := NewFileSet()
	fs.Add("/usr/bin/*")  // 添加通配符规则

	// 检查 "/usr/bin/gcc"
	IsInSetSmart("/usr/bin/gcc") 的处理过程：
	1. level=0: 检查 "/usr/bin/gcc"
	2. level=1: 检查 "/usr/bin"
	- 检查 "/usr/bin/*" <- 匹配！返回 true
	3. 如果前面没匹配，继续：
	level=2: 检查 "/usr"
	level=3: 检查 "/"
*/
func (s *FileSet) IsInSetSmart(name string) bool {
	if s.Set[name] {
		return true
	}
	if name == "/" && s.SystemRoot {
		return true
	}
	// 检查目录层级
	level := 0 
	for level = 0; name != ""; level++ {
		if level == 1 && s.Set[name+"/*"] {
			return true
		}
		if s.Set[name+"/"] {
			return true
		}
		name = dirname(name)
	}
	
	if level == 1 && s.Set["/*"] {
		return true
	}
	if s.Set["/"] {
		return true
	}
	return false
}

// Add 将单个文件路径添加到 FileSet
func (s *FileSet) Add(name string) {
	if name == "/" {
		s.SystemRoot = true
	} else {
		s.Set[name] = true
	}
}

// AddRange 将多个文件添加到 FileSet
// 如果路径是相对路径，则根据 workPath 添加
func (s *FileSet) AddRange(names []string, workPath string) {
	for _, n := range names {
		if filepath.IsAbs(n) {
			if n == "/" {
				s.SystemRoot = true
			} else {
				s.Set[n] = true
			}
		} else {
			s.Set[filepath.Join(workPath, n)+"/"] = true
		}
	}
}

// FileSets 聚合多个权限，包括写入/读取/状态/软禁止
// 普通禁止：返回 TraceKill（直接终止程序）
// 软禁止：返回 TraceBan（跳过系统调用，但允许程序继续运行）
type FileSets struct {
	Writable, Readable, Statable, SoftBan FileSet
}

// NewFileSets 创建新的 FileSets 结构
func NewFileSets() *FileSets {
	return &FileSets{NewFileSet(), NewFileSet(), NewFileSet(), NewFileSet()}
}

// IsWritableFile 判断文件路径是否在写入集合中
func (s *FileSets) IsWritableFile(name string) bool {
	return s.Writable.IsInSetSmart(name) || s.Writable.IsInSetSmart(realPath(name))
}

// IsReadableFile 判断文件路径是否在读取/写入集合中
func (s *FileSets) IsReadableFile(name string) bool {
	return s.IsWritableFile(name) || s.Readable.IsInSetSmart(name) || s.Readable.IsInSetSmart(realPath(name))
}

// IsStatableFile 判断文件路径是否在状态查看集合中
func (s *FileSets) IsStatableFile(name string) bool {
	return s.IsReadableFile(name) || s.Statable.IsInSetSmart(name) || s.Statable.IsInSetSmart(realPath(name))
}

// IsSoftBanFile 判断文件路径是否在软禁止集合中
func (s *FileSets) IsSoftBanFile(name string) bool {
	return s.SoftBan.IsInSetSmart(name) || s.SoftBan.IsInSetSmart(realPath(name))
}

// AddFilePermission 根据给定的权限将文件添加到 fileSets
func (s *FileSets) AddFilePermission(name string, mode FilePerm) {
	switch mode {
	case FilePermWrite:
		s.Writable.Add(name)
	case FilePermRead:
		s.Readable.Add(name)
	case FilePermStat:
		s.Statable.Add(name)
	}
}

// GetExtraSet 根据真实路径或原始路径评估连接的文件集
/*
	// 假设有以下情况：
	raw := []string{"/bin/python3"}
	extra := []string{"/usr/bin/python3"}  // 这可能是一个符号链接

	// 调用函数
	paths := GetExtraSet(extra, raw)

	GetExtraSet
	处理符号链接：
		如果一个文件有符号链接，它可能有多个路径指向同一个文件
		函数会处理这种情况，确保both路径都被正确处理
	去重处理：
		使用 map 确保每个路径只出现一次
		原始路径（raw）优先级高于额外路径（extra）
	路径规范化：
		通过 realPath 函数处理路径
		确保路径是规范的形式
*/
func GetExtraSet(extra, raw []string) []string {
    // 创建一个映射来存储唯一的路径
    m := make(map[string]bool)
    
    // 1. 首先添加所有原始路径（raw paths）
    for _, v := range raw {
        m[v] = true
    }
    
    // 2. 处理额外路径（extra paths）
    for _, v := range extra {
        if !m[v] {  // 如果原始路径中不存在
            m[realPath(v)] = true  // 添加其真实路径
        }
    }
    
    // 3. 转换回字符串数组
    ret := make([]string, 0, len(m))
    for k := range m {
        ret = append(ret, k)
    }
    return ret
}

// dirname 返回不带最后 "/" 的路径
func dirname(path string) string {
	if path == "" {
		return ""
	}
	// 去除最后的 "/"
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	// 返回父目录
	return filepath.Dir(path)
}

// realPath 获取真实路径
func realPath(p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	return filepath.Clean(p)
}
