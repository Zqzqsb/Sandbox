## 构建步骤

# Go Judge Sandbox 实现步骤

## 项目结构

```
reproduce_sanbox/
├── cmd/
│   └── runprog/         # 主程序入口
├── pkg/
│   ├── rlimit/         # 资源限制
│   ├── filehandler/    # 文件访问控制
│   ├── seccomp/        # 系统调用限制
│   ├── ptracer/        # ptrace 实现
│   ├── mount/          # 文件系统挂载
│   ├── namespace/      # namespace 隔离
│   ├── cgroup/         # cgroup 资源控制
│   ├── container/      # 容器实现
│   ├── memfd/          # 内存文件系统
│   └── unixsocket/     # Unix 域套接字通信
├── runner/             # 运行器接口定义
└── go.mod              # Go 模块定义

## 实现步骤

### 第一阶段：基础框架搭建

1. 创建基本项目结构
2. 实现 runner 接口
   - 定义基本的运行器接口
   - 实现结果状态定义
3. 实现资源限制 (rlimit)
   - CPU 时间限制
   - 内存限制
   - 堆栈限制
   - 输出限制
4. 实现文件访问控制
   - 读写权限控制
   - 文件路径检查

### 第二阶段：进程控制与系统调用限制

1. 实现 ptrace 跟踪
   - 系统调用跟踪
   - 进程状态监控
2. 实现 seccomp 过滤器
   - 系统调用白名单
   - 系统调用黑名单
   - 自定义规则支持

### 第三阶段：容器隔离

1. 实现 namespace 隔离
   - PID namespace
   - Mount namespace
   - Network namespace
2. 实现 cgroup 资源控制
   - CPU 限制
   - 内存限制
   - 进程数限制
3. 实现文件系统挂载
   - rootfs 准备
   - bind mount
   - tmpfs

### 第四阶段：高级特性

1. 实现容器池
   - 预创建容器
   - 容器复用
2. 实现 memfd 支持
   - 内存文件系统
   - 文件描述符传递
3. 实现进程间通信
   - Unix 域套接字
   - 管道通信

## 当前进度

- [x] 第一阶段
  - [x] 项目结构搭建
  - [x] runner 接口实现
    - [x] runner.go - 定义基本运行器接口
    - [x] result.go - 定义运行结果结构
    - [x] status.go - 定义状态枚举
    - [x] size.go - 实现内存大小处理
  - [x] 资源限制实现
    - [x] rlimit.go - 实现资源限制功能
  - [x] 文件访问控制
    - [x] filehandler/handle.go - 文件访问限制处理器
    - [x] filehandler/fileset.go - 文件权限集合
    - [x] filehandler/syscallcounter.go - 系统调用计数器

- [ ] 第二阶段：进程控制与系统调用限制
  - [x] ptrace 基础实现
    - [x] context_linux.go - 基本结构和接口
    - [x] context_linux_amd64.go - 架构特定实现
    - [x] context_helper_linux.go - 内存读取功能
      - [x] vmRead - 基础内存读取
      - [x] vmReadStr - 字符串读取
      - [x] hasNull - 辅助函数
    - [x] 完备的测试用例
      - [x] 基本功能测试
      - [x] 边界条件测试
      - [x] 内存对齐测试
  - [ ] ptrace 高级功能
    - [ ] 系统调用跟踪
    - [ ] 进程状态监控
    - [ ] 资源使用统计
  - [ ] seccomp 过滤器

## 下一步计划

1. 完善 ptrace 高级功能
   - 实现系统调用跟踪
   - 添加进程状态监控
   - 集成资源使用统计

2. 开发 seccomp 过滤器
   - 设计系统调用白名单
   - 实现过滤规则
   - 添加自定义规则支持

3. 强化测试覆盖
   - 添加更多边界条件测试
   - 压力测试和性能测试
   - 集成测试

## 注意事项

1. 所有代码实现需要与原始 sandbox 保持一致
2. 所有英文注释需要替换为中文注释
3. 保持相同的接口定义和功能实现
4. 确保兼容性和性能表现一致
5. 重点关注内存安全和边界检查