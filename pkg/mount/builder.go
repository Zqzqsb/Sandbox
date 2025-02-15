package mount

// Builder 构建适用于 fork_exec 的挂载系统调用格式
// 通过链式调用方式配置多个挂载点
type Builder struct {
	Mounts []Mount
}

// NewBuilder 创建一个新的挂载构建器实例
// 返回构建器指针以支持链式调用
func NewBuilder() *Builder {
	return &Builder{}
}
