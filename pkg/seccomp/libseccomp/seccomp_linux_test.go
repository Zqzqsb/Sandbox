package libseccomp

import (
	"testing"

	seccompbpf "github.com/elastic/go-seccomp-bpf"
	"github.com/zqzqsb/sandbox/pkg/seccomp"
)

var (
	defaultSyscallAllows = []string{
		"read", "write", "readv", "writev", "close", "fstat", "lseek", "dup", "dup2", "dup3", "ioctl", "fcntl", "fadvise64",
		"mmap", "mprotect", "munmap", "brk", "mremap", "msync", "mincore", "madvise",
		"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "rt_sigpending", "sigaltstack",
		"getcwd", "exit", "exit_group", "arch_prctl",
		"gettimeofday", "getrlimit", "getrusage", "times", "time", "clock_gettime", "restart_syscall",
	}

	defaultSyscallTraces = []string{
		"execve", "open", "openat", "unlink", "unlinkat", "readlink", "readlinkat", "lstat", "stat", "access", "faccessat",
	}
)

func TestBuildFilter(t *testing.T) {
	tests := []struct {
		name    string
		builder Builder
		wantErr bool
	}{
		{
			name: "basic",
			builder: Builder{
				Allow:   []string{"read", "write", "exit"},
				Trace:   []string{"open", "close"},
				Default: Action(seccomp.ActionKill),
			},
			wantErr: false,
		},
		{
			name: "empty allow list",
			builder: Builder{
				Trace:   []string{"open"},
				Default: Action(seccomp.ActionKill),
			},
			wantErr: false,
		},
		{
			name: "empty trace list",
			builder: Builder{
				Allow:   []string{"read"},
				Default: Action(seccomp.ActionKill),
			},
			wantErr: false,
		},
		{
			name: "invalid syscall",
			builder: Builder{
				Allow:   []string{"invalid_syscall"},
				Default: Action(seccomp.ActionKill),
			},
			wantErr: true,
		},
		{
			name: "duplicate syscalls",
			builder: Builder{
				Allow:   []string{"read", "read"},
				Default: Action(seccomp.ActionKill),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := tt.builder.Build()
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && filter == nil {
				t.Error("Builder.Build() returned nil filter without error")
			}
		})
	}
}

func TestToSeccompAction(t *testing.T) {
	tests := []struct {
		name string
		act  Action
		want seccompbpf.Action
	}{
		{
			name: "allow",
			act:  Action(seccomp.ActionAllow),
			want: seccompbpf.ActionAllow,
		},
		{
			name: "errno",
			act:  Action(seccomp.ActionErrno),
			want: seccompbpf.ActionErrno,
		},
		{
			name: "trace",
			act:  Action(seccomp.ActionTrace),
			want: seccompbpf.ActionTrace,
		},
		{
			name: "kill",
			act:  Action(99), // 无效动作
			want: seccompbpf.ActionKillProcess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToSeccompAction(tt.act); got != tt.want {
				t.Errorf("ToSeccompAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

// BenchmarkBuildFilter 测试过滤器构建的性能
func BenchmarkBuildFilter(b *testing.B) {
	builder := Builder{
		Allow:   defaultSyscallAllows,
		Trace:   defaultSyscallTraces,
		Default: Action(seccomp.ActionTrace),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func buildFilterMock() (seccomp.Filter, error) {
	b := Builder{
		Allow:   []string{"fork"},
		Trace:   []string{"execve"},
		Default: Action(seccomp.ActionTrace),
	}
	return b.Build()
}
