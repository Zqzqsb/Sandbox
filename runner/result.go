package runner

import (
	"fmt"
	"time"
)

// Result 是程序运行的结果
type Result struct {
	Status            // 结果状态
	ExitStatus int    // 退出状态（如果被信号终止则为信号编号）
	Error      string // 潜在的详细错误信息（用于程序运行器错误）

	Time   time.Duration // 使用的用户 CPU 时间（底层类型为 int64，单位纳秒）
	Memory Size          // 使用的用户内存（底层类型为 uint64，单位字节）

	// 程序运行器的度量指标
	SetUpTime   time.Duration // 设置时间
	RunningTime time.Duration // 运行时间
}


/*
	当一个类型实现了 String() 方法，它就自动实现了 fmt.Stringer 接口。这个接口的作用是：
	当使用 fmt.Print、fmt.Printf 或 fmt.Println 等函数打印该类型的值时，会自动调用 String() 方法来获取其字符串表示
	当使用 %v 或 %s 格式化指令时，也会调用 String() 方法
*/

func (r Result) String() string {
	switch r.Status {
	case StatusNormal:
		return fmt.Sprintf("Result[%v %v][%v %v]", r.Time, r.Memory, r.SetUpTime, r.RunningTime)

	case StatusSignalled:
		return fmt.Sprintf("Result[Signalled(%d)][%v %v][%v %v]", r.ExitStatus, r.Time, r.Memory, r.SetUpTime, r.RunningTime)

	case StatusRunnerError:
		return fmt.Sprintf("Result[RunnerFailed(%s)][%v %v][%v %v]", r.Error, r.Time, r.Memory, r.SetUpTime, r.RunningTime)

	default:
		return fmt.Sprintf("Result[%v(%s %d)][%v %v][%v %v]", r.Status, r.Error, r.ExitStatus, r.Time, r.Memory, r.SetUpTime, r.RunningTime)
	}
}
