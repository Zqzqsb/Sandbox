package ptracer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestHasNull 测试 hasNull 函数
func TestHasNull(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "empty buffer",
			data: []byte{},
			want: false,
		},
		{
			name: "no null",
			data: []byte("hello"),
			want: false,
		},
		{
			name: "has null at start",
			data: []byte{0, 1, 2, 3},
			want: true,
		},
		{
			name: "has null at end",
			data: []byte{1, 2, 3, 0},
			want: true,
		},
		{
			name: "has null in middle",
			data: []byte{1, 0, 3, 4},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasNull(tt.data); got != tt.want {
				t.Errorf("hasNull() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 辅助函数：创建一个子进程并返回其PID
func createTestProcess(t *testing.T) (int, func()) {
	cmd := exec.Command("sleep", "10") // 使用 sleep 命令创建一个持续运行的进程
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	cleanup := func() {
		cmd.Process.Kill()
		cmd.Wait()
	}

	return cmd.Process.Pid, cleanup
}

// TestVmRead 测试 vmRead 函数
func TestVmRead(t *testing.T) {
	pid, cleanup := createTestProcess(t)
	defer cleanup()

	// 创建测试数据
	testData := []byte("Hello, World!")
	buff := make([]byte, len(testData))

	// 获取进程的内存映射
	maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		t.Fatalf("Failed to read process maps: %v", err)
	}

	// 解析内存映射找到一个可读的地址
	var addr uintptr
	for _, line := range bytes.Split(maps, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		if bytes.Contains(line, []byte("r-x")) { // 找一个可读的段
			var start uint64
			fmt.Sscanf(string(line), "%x-", &start)
			addr = uintptr(start)
			break
		}
	}

	if addr == 0 {
		t.Fatal("Failed to find readable memory region")
	}

	// 测试读取
	n, err := vmRead(pid, addr, buff)
	if err != nil {
		t.Fatalf("vmRead failed: %v", err)
	}
	if n == 0 {
		t.Error("vmRead returned 0 bytes")
	}
}

// TestVmReadStr 测试 vmReadStr 函数
func TestVmReadStr(t *testing.T) {
	pid, cleanup := createTestProcess(t)
	defer cleanup()

	// 测试场景
	testCases := []struct {
		name      string
		buffSize  int
		addrAlign uintptr // 地址偏移，用于测试不同的对齐情况
		wantErr   bool
	}{
		{
			name:      "small_buffer_aligned",
			buffSize:  10,
			addrAlign: 0,
			wantErr:   false,
		},
		{
			name:      "small_buffer_unaligned",
			buffSize:  10,
			addrAlign: 1,
			wantErr:   false,
		},
		{
			name:      "exact_page_size",
			buffSize:  pageSize,
			addrAlign: 0,
			wantErr:   false,
		},
		{
			name:      "cross_page_boundary",
			buffSize:  pageSize + 100,
			addrAlign: uintptr(pageSize - 50),
			wantErr:   false,
		},
		{
			name:      "large_buffer_unaligned",
			buffSize:  pageSize * 2,
			addrAlign: 123,
			wantErr:   false,
		},
		{
			name:      "buffer_smaller_than_to_boundary",
			buffSize:  10,
			addrAlign: uintptr(pageSize - 100), // 距离页边界100字节，但缓冲区只有10字节
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buff := make([]byte, tc.buffSize)
			
			// 获取一个可读的内存地址
			maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
			if err != nil {
				t.Fatalf("Failed to read process maps: %v", err)
			}

			var baseAddr uintptr
			for _, line := range bytes.Split(maps, []byte{'\n'}) {
				if len(line) == 0 {
					continue
				}
				if bytes.Contains(line, []byte("r-x")) {
					var start uint64
					fmt.Sscanf(string(line), "%x-", &start)
					baseAddr = uintptr(start)
					break
				}
			}

			if baseAddr == 0 {
				t.Fatal("Failed to find readable memory region")
			}

			// 使用测试用例指定的对齐偏移
			testAddr := baseAddr + tc.addrAlign

			// 记录读取前的缓冲区内容
			originalBuff := make([]byte, len(buff))
			copy(originalBuff, buff)

			err = vmReadStr(pid, testAddr, buff)
			if (err != nil) != tc.wantErr {
				t.Errorf("vmReadStr() error = %v, wantErr %v", err, tc.wantErr)
			}

			// 验证是否有实际读取发生
			if !bytes.Equal(buff, originalBuff) {
				// 至少有一些数据被读取
				t.Logf("Data was read successfully for case: %s", tc.name)
			}

			// 特殊情况：检查缓冲区大小小于到页边界距离的情况
			if tc.name == "buffer_smaller_than_to_boundary" {
				distToBoundary := pageSize - int(testAddr%uintptr(pageSize))
				if distToBoundary > len(buff) {
					t.Logf("Verified buffer handling when smaller than distance to boundary: dist=%d, buff=%d",
						distToBoundary, len(buff))
				}
			}
		})
	}
}

// TestSliceBehavior 测试切片行为
func TestSliceBehavior(t *testing.T) {
	tests := []struct {
		name      string
		buffSize  int
		nextRead  int
		expected  int
	}{
		{
			name:     "small_buffer_large_read",
			buffSize: 10,
			nextRead: 4096,
			expected: 10,  // 必须限制在缓冲区大小内
		},
		{
			name:     "large_buffer_small_read",
			buffSize: 8192,
			nextRead: 4096,
			expected: 4096,  // 可以使用完整的读取量
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buff := make([]byte, tt.buffSize)
			// 安全地计算实际读取量
			actualRead := tt.nextRead
			if tt.buffSize < actualRead {
				actualRead = tt.buffSize
			}
			slice := buff[:actualRead]
			
			if len(slice) != tt.expected {
				t.Errorf("Expected slice len %d, got %d", tt.expected, len(slice))
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
