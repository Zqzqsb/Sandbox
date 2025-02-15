// Package pipe 提供了一个包装器，用于创建管道并从读取端收集最多指定字节数的数据
package pipe

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// Buffer 用于创建一个可写的管道，并将最多 max 字节的数据读取到缓冲区中
// 主要用于收集和限制程序的输出数据（如标准输出或标准错误）
type Buffer struct {
	W      *os.File       // 管道的写入端
	Buffer *bytes.Buffer  // 用于存储读取的数据的缓冲区
	Done   <-chan struct{} // 信号通道，当读取完成时关闭
	Max    int64          // 最大允许读取的字节数
}

// NewPipe 创建一个管道，并启动一个 goroutine 将其读取端的数据复制到指定的 writer
// 参数：
//   - writer: 数据写入的目标
//   - n: 最大复制的字节数
// 返回：
//   - <-chan struct{}: 完成信号通道，当复制完成时关闭
//   - *os.File: 管道的写入端
//   - error: 错误信息
// 注意：调用者需要负责关闭返回的写入端（w）
func NewPipe(writer io.Writer, n int64) (<-chan struct{}, *os.File, error) {
	// 创建一个新的操作系统管道
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	
	// 创建完成信号通道
	done := make(chan struct{})
	
	// 启动后台 goroutine 处理数据复制
	go func() {
		// 复制指定字节数的数据到 writer
		io.CopyN(writer, r, int64(n))
		// 复制完成后关闭信号通道
		close(done)
		// 继续读取并丢弃剩余数据，确保写入端不会因为管道满而阻塞或收到 SIGPIPE 信号
		io.Copy(io.Discard, r)
		// 关闭读取端
		r.Close()
	}()
	
	return done, w, nil
}

// NewBuffer 创建一个新的 Buffer，它包含一个 OS 管道和一个字节缓冲区
// 参数：
//   - max: 允许读取的最大字节数
// 返回：
//   - *Buffer: Buffer 实例
//   - error: 错误信息
// 注意：如果依赖 done 通道来判断完成，需要在父进程中关闭写入端
func NewBuffer(max int64) (*Buffer, error) {
	// 创建字节缓冲区
	buffer := new(bytes.Buffer)
	// 创建管道，最大读取字节数加1（可能用于检测是否超出限制）
	done, w, err := NewPipe(buffer, max+1)
	if err != nil {
		return nil, err
	}
	
	return &Buffer{
		W:      w,      // 管道写入端
		Max:    max,    // 最大字节数限制
		Buffer: buffer, // 数据缓冲区
		Done:   done,   // 完成信号通道
	}, nil
}

// String 实现 Stringer 接口，返回 Buffer 的当前状态字符串
// 格式为：Buffer[当前字节数/最大字节数]
func (b Buffer) String() string {
	return fmt.Sprintf("Buffer[%d/%d]", b.Buffer.Len(), b.Max)
}
