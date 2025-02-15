package forkexec

// 导入 unsafe 包是为了使用 go:linkname 指令
// go:linkname 允许我们链接到 runtime 包中的私有函数
import _ "unsafe"

// beforeFork 在执行 fork 之前被调用
// 它会：
// 1. 锁定所有线程，防止其他线程在 fork 期间运行
// 2. 刷新所有缓冲的 I/O
// 3. 保存当前的信号掩码
//
//go:linkname beforeFork syscall.runtime_BeforeFork
func beforeFork()

// afterFork 在父进程的 fork 操作完成后被调用
// 它会：
// 1. 恢复所有被锁定的线程
// 2. 恢复信号处理
// 3. 重新初始化运行时线程本地存储
//
//go:linkname afterFork syscall.runtime_AfterFork
func afterFork()

// afterForkInChild 在子进程中的 fork 操作完成后被调用
// 它会：
// 1. 重新初始化运行时状态
// 2. 清理不需要的线程状态
// 3. 设置子进程的信号处理
// 注意：在子进程中，只有当前线程存在，其他线程都不存在
//
//go:linkname afterForkInChild syscall.runtime_AfterForkInChild
func afterForkInChild()
