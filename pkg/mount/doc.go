/*
Package mount 提供了在 Linux 系统中管理挂载点的功能，特别适用于创建沙箱环境。

主要功能：

1. Mount 结构体：
   - 定义挂载点的基本属性（源、目标、文件系统类型等）
   - 支持转换为系统调用参数
   - 提供挂载点状态查询方法（只读、绑定挂载等）

2. Builder 模式：
   - 提供流式API来构建挂载点配置
   - 支持常见的挂载类型：
     * 绑定挂载（bind mount）
     * tmpfs文件系统
     * proc文件系统
   - 提供默认配置（/usr, /lib, /lib64, /bin）

3. 安全特性：
   - 支持只读挂载
   - 支持私有挂载（MS_PRIVATE）
   - 支持nosuid等安全标志

使用示例：

    builder := mount.NewDefaultBuilder().
        WithBind("/usr", "usr", true).      // 只读绑定挂载
        WithTmpfs("tmp", "size=64m").       // 创建临时文件系统
        WithProc()                          // 只读proc文件系统

    mounts, err := builder.Build()          // 构建挂载配置
*/
package mount
