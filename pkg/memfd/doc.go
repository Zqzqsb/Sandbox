// Package memfd 提供了 Linux memfd（内存文件）的接口，用于创建和密封内存中的文件。
// memfd 是一种特殊的文件系统，它允许在内存中创建匿名文件，这些文件可以被密封（sealed）以防止修改。
// 主要用于在沙箱环境中安全地共享只读文件。
// 
// 要求 Linux 内核版本 >= 3.17
package memfd
