package ptracer

import (
	"syscall"
	"unsafe"

	unix "golang.org/x/sys/unix"
)

// ptraceReadStr 使用 PTRACE_PEEKDATA 从目标进程内存中读取字符串
// 参数：
//   - pid: 目标进程的 ID
//   - addr: 要读取的内存地址
//   - buff: 用于存储读取数据的缓冲区
// 返回：
//   - error: 如果读取失败返回错误，成功返回 nil
// 注意：
//   - 使用 PtracePeekData 按字（word）读取数据
//   - 目标进程必须被 ptrace 附加
//   - buff 的大小决定了最大读取长度
//   - 读取在遇到 null 字节(\0)时停止

/*
	// ProcessVMReadv
	一次读取：
	[====== 4096 bytes ======]  // 一次系统调用

	// PTRACE_PEEKDATA
	多次读取：
	[4 bytes][4 bytes][4 bytes]...  // 多次系统调用 ptracePeekText
*/

func ptraceReadStr(pid int, addr uintptr, buff []byte) error {
	_, err := syscall.PtracePeekData(pid, addr, buff)
	return err
}

// processVMReadv 封装 process_vm_readv 系统调用，用于在进程间直接传输数据
// 相比 ptrace，这是一种更高效的进程间内存读取方式
//
// 参数：
//   - pid: 目标进程的 ID
//   - localIov: 本地进程的内存向量数组，指定接收数据的缓冲区
//   - remoteIov: 远程进程的内存向量数组，指定要读取的内存区域
//   - flags: 控制标志位，当前必须为 0
//
// 返回：
//   - r1: 成功读取的字节数
//   - r2: 保留值（通常为0）
//   - err: 系统调用错误码，0 表示成功
//
// 注意：
//   - 需要 Linux 3.2+ 内核支持
//   - 比 PTRACE_PEEKDATA 更高效，因为：
//     1. 一次系统调用可以读取多个不连续的内存区域
//     2. 不需要目标进程处于 ptrace-stop 状态
//     3. 减少了上下文切换
//
// 系统调用格式：
// ssize_t process_vm_readv(pid_t pid,
//                         const struct iovec *local_iov,
//                         unsigned long liovcnt,
//                         const struct iovec *remote_iov,
//                         unsigned long riovcnt,
//                         unsigned long flags);

func processVMReadv(pid int, localIov, remoteIov []unix.Iovec,
	flags uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return syscall.Syscall6(unix.SYS_PROCESS_VM_READV, uintptr(pid),
		uintptr(unsafe.Pointer(&localIov[0])), uintptr(len(localIov)),
		uintptr(unsafe.Pointer(&remoteIov[0])), uintptr(len(remoteIov)),
		flags)
}

// vmRead 使用 process_vm_readv 从目标进程内存中读取数据
// 这是对 processVMReadv 的高层封装，简化了内存向量的创建和使用
//
// 参数：
//   - pid: 目标进程的 ID
//   - addr: 要读取的内存地址
//   - buff: 用于存储读取数据的缓冲区
//
// 返回：
//   - int: 实际读取的字节数
//   - error: 如果读取失败返回错误，成功返回 nil
//
// 工作流程：
//  1. 创建本地和远程内存向量（iovec）
//  2. 调用 process_vm_readv 系统调用
//  3. 处理返回值和错误
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

// getIovecs 创建 iovec 数组
func getIovecs(base *byte, l int) []unix.Iovec {
	return []unix.Iovec{getIovec(base, l)}
}

// getIovec 创建单个 iovec
func getIovec(base *byte, l int) unix.Iovec {
	return unix.Iovec{Base: base, Len: uint64(l)}
}

// vmReadStr 使用 process_vm_readv 从目标进程内存中读取字符串
// 该函数处理内存对齐问题，并按页大小分块读取数据
//
// 参数：
//   - pid: 目标进程的 ID
//   - addr: 要读取的内存地址
//   - buff: 用于存储读取数据的缓冲区
//
// 返回：
//   - error: 如果读取失败返回错误，成功返回 nil
//
// 工作原理：
//   1. 处理内存对齐：计算到页边界的偏移
//   2. 分块读取：按页大小读取数据
//   3. 遇到以下情况停止：
//      - 读取到 null 字节(\0)
//      - 缓冲区已满
//      - 发生错误
//      - 读取返回 0（表示结束）

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

// hasNull 检查缓冲区中是否包含 null 字符
func hasNull(buff []byte) bool {
	for _, v := range buff {
		if v == 0 {
			return true
		}
	}
	return false
}

// clen 返回 C 风格字符串的长度（到第一个 null 字符）
// func clen(b []byte) int {
// 	for i := 0; i < len(b); i++ {
// 		if b[i] == 0 {
// 			return i
// 		}
// 	}
// 	return len(b)
// }
