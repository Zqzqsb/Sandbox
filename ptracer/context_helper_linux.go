package ptracer

import (
	"syscall"
	"unsafe"

	unix "golang.org/x/sys/unix"
)

/* ptraceReadStr 使用 PTRACE_PEEKDATA 从目标进程内存中读取字符串

ptraceReadStr 通过 PTRACE_PEEKDATA 从目标进程内存中读取字符串。它使用 ptrace
系统调用来读取目标进程的内存数据。

实现细节：
  1. 使用 PTRACE_PEEKDATA 读取目标进程的内存数据
  2. 将读取的数据存储到缓冲区中

参数：
  - pid: 目标进程的 ID
  - addr: 要读取的内存地址
  - buff: 用于存储读取数据的缓冲区

返回值：
  - error: 如果读取失败返回错误，成功返回 nil

注意事项：
  1. 目标进程必须被 ptrace 附加
  2. buff 的大小决定了最大读取长度
  3. 读取在遇到 null 字节(\0)时停止 */

func ptraceReadStr(pid int, addr uintptr, buff []byte) error {
	_, err := syscall.PtracePeekData(pid, addr, buff)
	return err
}

/* processVMReadv 封装 process_vm_readv 系统调用，用于在进程间直接传输数据

processVMReadv 是对 Linux process_vm_readv 系统调用的封装，提供了一种高效的
进程间内存读取机制。相比传统的 ptrace 方式，它具有更好的性能和灵活性。

系统调用格式：
  ssize_t process_vm_readv(pid_t pid,
                          const struct iovec *local_iov,
                          unsigned long liovcnt,
                          const struct iovec *remote_iov,
                          unsigned long riovcnt,
                          unsigned long flags);

参数：
  - pid: 目标进程的 ID
  - localIov: 本地进程的内存向量数组，指定接收数据的缓冲区
  - remoteIov: 远程进程的内存向量数组，指定要读取的内存区域
  - flags: 控制标志位，当前必须为 0

返回值：
  - r1: 成功读取的字节数
  - r2: 保留值（通常为0）
  - err: 系统调用错误码，0 表示成功

性能优势：
  1. 一次系统调用可以读取多个不连续的内存区域
  2. 不需要目标进程处于 ptrace-stop 状态
  3. 减少了上下文切换的开销

使用要求：
  1. 需要 Linux 3.2+ 内核支持
  2. 调用进程需要适当的权限
  3. 目标进程必须存在且可访问
  4. 避免跨页面边界读取 */

func processVMReadv(pid int, localIov, remoteIov []unix.Iovec,
	flags uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return syscall.Syscall6(unix.SYS_PROCESS_VM_READV, uintptr(pid),
		uintptr(unsafe.Pointer(&localIov[0])), uintptr(len(localIov)),
		uintptr(unsafe.Pointer(&remoteIov[0])), uintptr(len(remoteIov)),
		flags)
}

/* vmRead 使用 process_vm_readv 从目标进程内存中读取数据

vmRead 是对 processVMReadv 的高层封装，简化了内存向量的创建和使用。

实现细节：
  1. 创建本地和远程内存向量（iovec）
  2. 调用 process_vm_readv 系统调用
  3. 处理返回值和错误

参数：
  - pid: 目标进程的 ID
  - addr: 要读取的内存地址
  - buff: 用于存储读取数据的缓冲区

返回值：
  - int: 实际读取的字节数
  - error: 如果读取失败返回错误，成功返回 nil

注意事项：
  1. buff 的大小决定了最大读取长度
  2. 读取在遇到 null 字节(\0)时停止 */

func vmRead(pid int, addr uintptr, buff []byte) (int, error) {
	l := len(buff)
	// 创建本地内存向量，指向接收缓冲区
	localIov := getIovecs(&buff[0], l)
	// 创建远程内存向量，指向目标进程的内存
	remoteIov := getIovecs((*byte)(unsafe.Pointer(addr)), l)
	// 执行内存读取
	n, _, err := processVMReadv(pid, localIov, remoteIov, uintptr(0))
	if err == 0 {
		return int(n), nil
	}
	return int(n), err
}

/* getIovecs 创建 iovec 数组

getIovecs 创建一个 iovec 数组，用于指定内存读取的源和目的地址。

实现细节：
  1. 创建 iovec 结构体
  2. 初始化 iovec 结构体的 Base 和 Len 字段

参数：
  - base: 内存地址的基址
  - l: 内存长度

返回值：
  - []unix.Iovec: 创建的 iovec 数组 */

func getIovecs(base *byte, l int) []unix.Iovec {
	return []unix.Iovec{getIovec(base, l)}
}

/* getIovec 创建单个 iovec

getIovec 创建一个单独的 iovec 结构体，用于指定内存读取的源和目的地址。

实现细节：
  1. 创建 iovec 结构体
  2. 初始化 iovec 结构体的 Base 和 Len 字段

参数：
  - base: 内存地址的基址
  - l: 内存长度

返回值：
  - unix.Iovec: 创建的 iovec 结构体 */

func getIovec(base *byte, l int) unix.Iovec {
	return unix.Iovec{Base: base, Len: uint64(l)}
}

/* vmReadStr 使用 process_vm_readv 从目标进程内存中读取字符串

vmReadStr 通过 process_vm_readv 从目标进程读取以 null 结尾的字符串。它会
分页读取数据以避免跨页面访问可能带来的问题。

实现细节：
  1. 按页面大小分块读取
  2. 检测字符串结束符（null）
  3. 处理内存对齐和边界

参数：
  - pid: 目标进程的ID
  - addr: 字符串在目标进程中的地址
  - buff: 用于存储读取数据的缓冲区

返回值：
  - error: 如果读取失败返回错误，成功返回 nil

注意事项：
  1. 字符串必须以 null 结尾
  2. 避免跨页面读取
  3. 处理读取长度限制 */

func vmReadStr(pid int, addr uintptr, buff []byte) error {
	// 处理未对齐的地址：计算到页边界的剩余字节数
	totalRead := 0 // 已读取的总字节数
	// 计算到下一个页边界的距离 nextRead 是每次要读取的字节数目
	nextRead := pageSize - int(addr%uintptr(pageSize))
	if nextRead == 0 {
		nextRead = pageSize // 如果正好在页边界，则使用整页大小
	}

	// 循环读取，直到缓冲区填满或遇到终止条件
	for len(buff) > 0 {
		// 如果剩余缓冲区小于计划读取量，则减小读取量
		if restToRead := len(buff); restToRead < nextRead {
			nextRead = restToRead
		}

		// 从当前位置读取数据
		curRead, err := vmRead(pid, addr+uintptr(totalRead), buff[:nextRead])
		if err != nil {
			return err // 读取错误
		}
		if curRead == 0 {
			break // 没有更多数据可读
		}
		if hasNull(buff[:curRead]) {
			break // 找到字符串结束符
		}

		// 更新计数器和缓冲区
		totalRead += curRead  // 更新总读取量
		buff = buff[curRead:] // 移动缓冲区指针
		nextRead = pageSize   // 重置为完整页大小
	}
	return nil
}

/* hasNull 检查缓冲区中是否包含 null 字符

hasNull 检查缓冲区中是否包含 null 字符（\0）。

实现细节：
  1. 遍历缓冲区中的每个字节
  2. 检测 null 字符

参数：
  - buff: 要检查的缓冲区

返回值：
  - bool: 如果缓冲区中包含 null 字符，则返回 true，否则返回 false */

func hasNull(buff []byte) bool {
	for _, v := range buff {
		if v == 0 {
			return true
		}
	}
	return false
}
