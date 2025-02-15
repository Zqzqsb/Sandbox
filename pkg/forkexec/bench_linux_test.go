package forkexec

import (
	"os"
	"syscall"
	"testing"

	"github.com/zqzqsb/sandbox/pkg/mount"
	"golang.org/x/sys/unix"
)

// 所有测试数据来自 amd64 架构的 docker 环境

const (
	// 只读绑定挂载的标志组合
	// MS_BIND: 创建绑定挂载
	// MS_NOSUID: 禁止 setuid/setgid 位
	// MS_PRIVATE: 私有挂载
	// MS_RDONLY: 只读模式
	roBind = unix.MS_BIND | unix.MS_NOSUID | unix.MS_PRIVATE | unix.MS_RDONLY
)

var (
	// 默认需要绑定挂载的系统目录
	defaultBind = []string{"/usr", "/lib", "/lib64", "/bin"}
)

// BenchmarkStdFork 测试标准库的 ForkExec 性能
func BenchmarkStdFork(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pid, err := syscall.ForkExec("/usr/bin/true", []string{"true"}, &syscall.ProcAttr{
				Env: []string{"PATH=/usr/bin:/bin"},
				Files: []uintptr{
					uintptr(syscall.Stdin),
					uintptr(syscall.Stdout),
					uintptr(syscall.Stderr),
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			wait4(pid, b)
		}
	})
}

// BenchmarkSimpleFork 测试基本的 fork 操作
// 大约耗时 0.70ms/op
func BenchmarkSimpleFork(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	benchmarkRun(r, b)
}

// BenchmarkUnsharePid 测试 PID 命名空间隔离
// 大约耗时 0.79ms/op
func BenchmarkUnsharePid(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWPID
	benchmarkRun(r, b)
}

// BenchmarkUnshareUser 测试用户命名空间隔离
// 大约耗时 0.84ms/op
func BenchmarkUnshareUser(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWUSER
	benchmarkRun(r, b)
}

// BenchmarkUnshareUts 测试 UTS 命名空间隔离
// 大约耗时 0.78ms/op
func BenchmarkUnshareUts(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWUTS
	benchmarkRun(r, b)
}

// BenchmarkUnshareCgroup 测试 Cgroup 命名空间隔离
// 大约耗时 0.85ms/op
func BenchmarkUnshareCgroup(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWCGROUP
	benchmarkRun(r, b)
}

// BenchmarkUnshareIpc 测试 IPC 命名空间隔离
// 大约耗时 51ms/op
func BenchmarkUnshareIpc(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWIPC
	benchmarkRun(r, b)
}

// BenchmarkUnshareMount 测试挂载命名空间隔离
// 大约耗时 51ms/op
func BenchmarkUnshareMount(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWNS
	benchmarkRun(r, b)
}

// BenchmarkUnshareNet 测试网络命名空间隔离
// 大约耗时 426ms/op
func BenchmarkUnshareNet(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = unix.CLONE_NEWNET
	benchmarkRun(r, b)
}

// BenchmarkFastUnshareMountPivot 测试快速的挂载隔离和根文件系统切换
// 仅隔离必要的命名空间，大约耗时 104ms/op
func BenchmarkFastUnshareMountPivot(b *testing.B) {
	// 创建临时目录作为新的根文件系统
	root, err := os.MkdirTemp("", "ns")
	if err != nil {
		b.Errorf("failed to create temp dir")
	}
	defer os.RemoveAll(root)
	r, f := getRunner(b)
	defer f.Close()
	// 只隔离必要的命名空间
	r.CloneFlags = unix.CLONE_NEWNS | unix.CLONE_NEWPID | unix.CLONE_NEWUSER | unix.CLONE_NEWUTS | unix.CLONE_NEWCGROUP
	r.PivotRoot = root
	r.NoNewPrivs = true
	r.DropCaps = true
	r.Mounts = getMounts(defaultBind)
	benchmarkRun(r, b)
}

// BenchmarkUnshareAll 测试完全命名空间隔离
// 大约耗时 800ms/op
func BenchmarkUnshareAll(b *testing.B) {
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = UnshareFlags
	r.NoNewPrivs = true
	r.DropCaps = true
	benchmarkRun(r, b)
}

// BenchmarkUnshareMountPivot 测试完全命名空间隔离并切换根文件系统
// 大约耗时 880ms/op
func BenchmarkUnshareMountPivot(b *testing.B) {
	// 创建临时目录作为新的根文件系统
	root, err := os.MkdirTemp("", "ns")
	if err != nil {
		b.Errorf("failed to create temp dir")
	}
	defer os.RemoveAll(root)
	r, f := getRunner(b)
	defer f.Close()
	r.CloneFlags = UnshareFlags
	r.PivotRoot = root
	r.NoNewPrivs = true
	r.DropCaps = true
	r.Mounts = getMounts(defaultBind)
	benchmarkRun(r, b)
}

// getRunner 创建一个基本的 Runner 实例
// 将标准输入、输出、错误都重定向到 /dev/null
func getRunner(b *testing.B) (*Runner, *os.File) {
	f := openNull(b)
	return &Runner{
		Args:    []string{"/bin/echo"},
		Env:     []string{"PATH=/bin"},
		Files:   []uintptr{f.Fd(), f.Fd(), f.Fd()},
		WorkDir: "/bin",
	}, f
}

// benchmarkRun 执行并行基准测试
// 重复创建进程并等待其完成
func benchmarkRun(r *Runner, b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pid, err := r.Start()
			if err != nil {
				b.Fatal(err)
			}
			wait4(pid, b)
		}
	})
}

// getMounts 根据给定的目录列表创建挂载配置
// 将目录以只读方式绑定挂载到新的根文件系统中
func getMounts(dirs []string) []mount.SyscallParams {
	builder := mount.NewBuilder()
	for _, d := range dirs {
		builder.WithMount(mount.Mount{
			Source: d,            // 源目录
			Target: d[1:],       // 目标目录（去掉开头的 /）
			Flags:  roBind,      // 只读绑定挂载
		})
	}
	m, _ := builder.FilterNotExist().Build()
	return m
}

// openNull 打开 /dev/null 文件
// 用于重定向进程的标准输入输出
func openNull(b *testing.B) *os.File {
	f, err := os.OpenFile("/dev/null", os.O_RDWR, 0666)
	if err != nil {
		b.Errorf("Failed to open %v", err)
	}
	return f
}

// wait4 等待指定 PID 的进程结束
// 检查进程的退出状态，非零退出码视为错误
func wait4(pid int, b *testing.B) {
	var wstat syscall.WaitStatus
	for {
		syscall.Wait4(pid, &wstat, 0, nil)
		if wstat.Exited() {
			if s := wstat.ExitStatus(); s != 0 {
				b.Errorf("Exited: %d", s)
			}
			break
		}
	}
}
