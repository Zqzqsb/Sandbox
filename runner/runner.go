// Package runner 提供了程序运行的基本接口定义
package runner

import (
	"context"
)

// Runner 接口定义了启动运行的方法
type Runner interface {
	Run(context.Context) Result
}
