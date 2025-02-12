# handle 函数工作流程

```mermaid
flowchart TD
    Start([开始]) --> Init[初始化状态\nstatus = StatusNormal]
    Init --> Check{检查进程状态}
    
    %% 退出分支
    Check -->|Exited| RemoveE[从traced移除进程]
    RemoveE --> IsMainE{是主进程?}
    IsMainE -->|否| Return
    IsMainE -->|是| SetFinished[finished = true]
    SetFinished --> ExecCheck{已执行exec?}
    ExecCheck -->|是| CheckExit{退出码=0?}
    CheckExit -->|是| SetNormal[status = StatusNormal]
    CheckExit -->|否| SetNonZero[status = StatusNonzeroExitStatus]
    ExecCheck -->|否| SetError1[status = StatusRunnerError\n'exited before execve']
    
    %% 信号终止分支
    Check -->|Signaled| RemoveS[从traced移除进程]
    RemoveS --> IsMainS{是主进程?}
    IsMainS -->|否| Return
    IsMainS -->|是| SetSignal[status = StatusSignalled\nfinished = true]
    
    %% 停止分支
    Check -->|Stopped| IsTraced{是否已跟踪?}
    IsTraced -->|否| AddTrace[添加到traced]
    AddTrace --> SetOption[设置ptrace选项]
    SetOption --> CheckError{设置成功?}
    CheckError -->|否| SetError2[status = StatusRunnerError]
    CheckError -->|是| CheckMain{主进程且未exec?}
    CheckMain -->|是| SetTime[记录开始时间]
    CheckMain -->|否| GetSignal[获取停止信号]
    SetTime --> GetSignal
    
    GetSignal --> IsTrap{是SIGTRAP?}
    IsTrap -->|否| Continue
    IsTrap -->|是| GetEvent[获取事件类型]
    
    GetEvent --> EventType{事件类型}
    EventType -->|Seccomp| HandleTrap[处理系统调用]
    EventType -->|Exec| SetExec[设置execved=true]
    EventType -->|Clone/Fork| LogEvent[记录创建事件]
    EventType -->|其他| LogTrap[记录Trap事件]
    
    HandleTrap --> CheckTrap{处理成功?}
    CheckTrap -->|否| SetError3[status = StatusRunnerError]
    CheckTrap -->|是| Continue
    SetExec --> Continue
    LogEvent --> Continue
    LogTrap --> Continue
    
    Continue[继续进程执行] --> CheckCont{继续成功?}
    CheckCont -->|否| SetError4[status = StatusRunnerError]
    CheckCont -->|是| Return
    
    IsTraced -->|是| GetSignal
    
    Return([返回])
    
    %% 样式
    classDef error fill:#ffcccc
    classDef success fill:#ccffcc
    classDef process fill:#cce5ff
    
    class SetError1,SetError2,SetError3,SetError4 error
    class SetNormal,Continue success
    class HandleTrap,SetOption process
```

## 关键节点说明

### 1. 状态检查分支
```
Exited：进程正常退出
- 检查是否是主进程
- 检查是否已执行 exec
- 设置相应的退出状态

Signaled：进程被信号终止
- 检查是否是主进程
- 设置信号终止状态

Stopped：进程停止
- 设置跟踪选项
- 处理各种事件
```

### 2. 事件处理
```
PTRACE_EVENT_SECCOMP：系统调用过滤
PTRACE_EVENT_EXEC：执行新程序
PTRACE_EVENT_CLONE/FORK：创建新进程
其他 TRAP 事件
```

### 3. 返回状态
```
StatusNormal：正常执行
StatusRunnerError：运行时错误
StatusNonzeroExitStatus：非零退出
StatusSignalled：信号终止
```

### 4. 错误处理点
```
1. ptrace 选项设置失败
2. exec 前进程退出
3. seccomp 处理失败
4. 进程继续执行失败
```

### 5. 重要标记
```
finished：是否完成
execved：是否执行了新程序
traced：进程是否被跟踪
```

## 关键流程

1. 进程生命周期：
```
fork -> traced添加 -> exec -> 执行 -> 退出
```

2. 事件处理：
```
停止 -> 获取事件 -> 处理 -> 继续执行
```

3. 错误处理：
```
检测错误 -> 设置状态 -> 返回错误信息
```
