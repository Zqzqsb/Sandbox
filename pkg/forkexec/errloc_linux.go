// Package forkexec 提供进程创建和执行的功能
package forkexec

import (
	"fmt"
	"syscall"
)

// ErrorLocation 定义了子进程执行失败的具体位置
// 这个类型用于精确定位在进程创建和配置过程中的哪个步骤出现了错误
type ErrorLocation int

// ChildError 定义了子进程错误的详细信息
// 包含三个字段：
// - Err: 系统调用返回的错误码
// - Location: 错误发生的位置
// - Index: 在某些情况下（如挂载操作）表示操作的序号
type ChildError struct {
	Err      syscall.Errno    // 系统调用错误码
	Location ErrorLocation    // 错误发生的位置
	Index    int             // 操作序号（如果适用）
}

// Location 常量定义了所有可能的错误位置
// 这些常量按照子进程初始化和配置的顺序排列
const (
	LocClone ErrorLocation = iota + 1            // 克隆（创建）新进程失败
	LocCloseWrite                                // 关闭写入端失败
	LocUnshareUserRead                           // 读取用户命名空间配置失败
	LocGetPid                                    // 获取进程 ID 失败
	LocKeepCapability                            // 保持进程能力失败
	LocSetGroups                                 // 设置用户组失败
	LocSetGid                                    // 设置组 ID 失败
	LocSetUid                                    // 设置用户 ID 失败
	LocDup3                                      // 复制文件描述符失败
	LocFcntl                                     // 文件控制操作失败
	LocSetSid                                    // 设置会话 ID 失败
	LocIoctl                                     // IO 控制操作失败
	LocMountRoot                                 // 挂载根文件系统失败
	LocMountTmpfs                                // 挂载临时文件系统失败
	LocMountChdir                                // 切换目录后挂载失败
	LocMount                                     // 常规挂载操作失败
	LocMountMkdir                                // 创建挂载点目录失败
	LocPivotRoot                                 // 切换根目录失败
	LocUmount                                    // 卸载文件系统失败
	LocUnlink                                    // 删除文件失败
	LocMountRootReadonly                         // 将根文件系统重新挂载为只读失败
	LocChdir                                     // 改变工作目录失败
	LocSetRlimit                                 // 设置资源限制失败
	LocSetNoNewPrivs                             // 禁止获取新特权失败
	LocDropCapability                            // 删除进程能力失败
	LocSetCap                                    // 设置进程能力失败
	LocPtraceMe                                  // 启用 ptrace 跟踪失败
	LocStop                                      // 停止进程失败
	LocSeccomp                                   // 配置 seccomp 失败
	LocSyncWrite                                 // 同步写入失败
	LocSyncRead                                  // 同步读取失败
	LocExecve                                    // 执行新程序失败
)

// locToString 将错误位置常量映射为人类可读的字符串
// 数组索引对应 ErrorLocation 的值
var locToString = []string{
	"unknown",                // 0: 未知位置
	"clone",                 // 1: 克隆进程
	"close_write",          // 2: 关闭写入
	"unshare_user_read",    // 3: 读取用户命名空间
	"getpid",               // 4: 获取进程ID
	"keep_capability",      // 5: 保持能力
	"setgroups",           // 6: 设置用户组
	"setgid",              // 7: 设置组ID
	"setuid",              // 8: 设置用户ID
	"dup3",                // 9: 复制文件描述符
	"fcntl",               // 10: 文件控制
	"setsid",              // 11: 设置会话ID
	"ioctl",               // 12: IO控制
	"mount(root)",         // 13: 挂载根文件系统
	"mount(tmpfs)",        // 14: 挂载临时文件系统
	"mount(chdir)",        // 15: 切换目录后挂载
	"mount",               // 16: 常规挂载
	"mount(mkdir)",        // 17: 创建挂载点
	"pivot_root",          // 18: 切换根目录
	"umount",              // 19: 卸载文件系统
	"unlink",              // 20: 删除文件
	"mount(readonly)",     // 21: 只读挂载
	"chdir",               // 22: 改变目录
	"setrlimt",            // 23: 设置资源限制
	"set_no_new_privs",    // 24: 禁止新特权
	"drop_capability",     // 25: 删除能力
	"set_cap",             // 26: 设置能力
	"ptrace_me",           // 27: 启用ptrace
	"stop",                // 28: 停止进程
	"seccomp",             // 29: 配置seccomp
	"sync_write",          // 30: 同步写入
	"sync_read",           // 31: 同步读取
	"execve",              // 32: 执行程序
}

// String 将 ErrorLocation 转换为人类可读的字符串
// 如果位置值在有效范围内，返回对应的描述
// 否则返回 "unknown"
func (e ErrorLocation) String() string {
	if e >= LocClone && e <= LocExecve {
		return locToString[e]
	}
	return "unknown"
}

// Error 实现了 error 接口，提供格式化的错误信息
// 如果 Index > 0（通常是在挂载操作中），将包含索引信息
// 例如：
// - "mount: permission denied" （无索引）
// - "mount(2): device busy" （有索引）
func (e ChildError) Error() string {
	if e.Index > 0 {
		return fmt.Sprintf("%s(%d): %s", e.Location.String(), e.Index, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Location.String(), e.Err.Error())
}
