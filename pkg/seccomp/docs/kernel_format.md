# 内核过滤器格式说明

## SockFilter vs SockFprog

### SockFilter 结构

`syscall.SockFilter` 是单个 BPF 指令的表示形式：

```go
type SockFilter struct {
    Code uint16    // 操作码
    Jt   uint8     // true 跳转偏移
    Jf   uint8     // false 跳转偏移
    K    uint32    // 通用字段（立即数/内存地址）
}
```

#### 字段说明
1. **Code**：
   - BPF 虚拟机的操作码
   - 定义了指令的行为（加载、存储、跳转等）
   - 例如：`BPF_LD`（加载）、`BPF_JMP`（跳转）

2. **Jt/Jf**：
   - 条件跳转的目标偏移量
   - Jt：条件为真时跳转的目标
   - Jf：条件为假时跳转的目标
   - 用于实现分支逻辑

3. **K**：
   - 多用途字段
   - 可以是立即数值
   - 可以是内存地址
   - 可以是比较值

### SockFprog 结构

`syscall.SockFprog` 是完整的 BPF 程序结构：

```go
type SockFprog struct {
    Len    uint16          // 过滤器的长度（指令数量）
    Filter *SockFilter     // 指向过滤器数组的指针
}
```

#### 字段说明
1. **Len**：
   - 程序包含的指令数量
   - 用于内核知道需要执行多少条指令
   - 防止越界访问

2. **Filter**：
   - 指向 SockFilter 数组的指针
   - 数组必须是连续的内存区域
   - 内核通过这个指针访问指令序列

## 关键区别

1. **抽象层次**：
   - SockFilter：单条指令级别
   - SockFprog：程序级别

2. **使用场景**：
   - SockFilter：构建具体的过滤规则
   - SockFprog：向内核提交完整的程序

3. **内存布局**：
   - SockFilter：独立的指令结构
   - SockFprog：连续的内存区域

4. **功能职责**：
   - SockFilter：定义具体的过滤行为
   - SockFprog：组织和管理过滤器集合

## 实际应用

```go
// 1. 创建过滤器指令
filters := []syscall.SockFilter{
    {Code: BPF_LD | BPF_W | BPF_ABS, K: 4},     // 加载系统调用号
    {Code: BPF_JMP | BPF_JEQ | BPF_K, K: 1, Jt: 0, Jf: 1},  // 比较
    {Code: BPF_RET | BPF_K, K: SECCOMP_RET_ALLOW},  // 允许
    {Code: BPF_RET | BPF_K, K: SECCOMP_RET_KILL},   // 禁止
}

// 2. 创建程序结构
prog := &syscall.SockFprog{
    Len: uint16(len(filters)),
    Filter: &filters[0],
}

// 3. 应用到进程
syscall.Prctl(syscall.PR_SET_SECCOMP, 
    uintptr(SECCOMP_MODE_FILTER), 
    uintptr(unsafe.Pointer(prog)))
```

## 性能考虑

1. **内存布局**：
   - SockFilter 数组必须是连续的
   - 避��在执行时的内存分配
   - 提高指令获取效率

2. **指令优化**：
   - 合理使用跳转减少指令数量
   - 优化条件判断的顺序
   - 减少内存访问

3. **缓存友好**：
   - 连续的内存布局有利于 CPU 缓存
   - 提高指令预取的效率
   - 减少缓存未命中
