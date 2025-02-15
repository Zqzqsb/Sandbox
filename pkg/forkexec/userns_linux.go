package forkexec

import (
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// writeIDMaps 为用户命名空间中的进程写入用户 ID 和组 ID 映射
// 此函数由父进程调用，用于设置子进程的 ID 映射
// 参数：
//   - r: Runner 结构体，包含 UID/GID 映射配置
//   - pid: 目标进程的 PID
func writeIDMaps(r *Runner, pid int) error {
	var uidMappings, gidMappings, setGroups []byte
	pidStr := strconv.Itoa(pid)

	// 处理 UID 映射
	// 如果没有指定映射，则创建默认映射：
	// 将容器内的 UID 0 映射到主机上的当前有效用户 ID
	if r.UIDMappings == nil {
		uidMappings = []byte("0 " + strconv.Itoa(unix.Geteuid()) + " 1")
	} else {
		// 使用用户指定的 UID 映射
		uidMappings = formatIDMappings(r.UIDMappings)
	}
	// 写入 UID 映射到 /proc/[pid]/uid_map
	if err := writeFile("/proc/"+pidStr+"/uid_map", uidMappings); err != nil {
		return err
	}

	// 设置 setgroups 的权限
	// setgroups 文件控制进程是否可以调用 setgroups 系统调用
	// 在使用用户命名空间时，必须先写入此文件再设置 GID 映射
	if r.GIDMappings == nil || !r.GIDMappingsEnableSetgroups {
		// 如果没有 GID 映射或禁用了 setgroups，则拒绝 setgroups 操作
		setGroups = setGIDDeny
	} else {
		// 否则允许 setgroups 操作
		setGroups = setGIDAllow
	}
	// 写入 setgroups 设置到 /proc/[pid]/setgroups
	if err := writeFile("/proc/"+pidStr+"/setgroups", setGroups); err != nil {
		return err
	}

	// 处理 GID 映射
	// 如果没有指定映射，则创建默认映射：
	// 将容器内的 GID 0 映射到主机上的当前有效组 ID
	if r.GIDMappings == nil {
		gidMappings = []byte("0 " + strconv.Itoa(unix.Getegid()) + " 1")
	} else {
		// 使用用户指定的 GID 映射
		gidMappings = formatIDMappings(r.GIDMappings)
	}
	// 写入 GID 映射到 /proc/[pid]/gid_map
	if err := writeFile("/proc/"+pidStr+"/gid_map", gidMappings); err != nil {
		return err
	}
	return nil
}

// formatIDMappings 将 ID 映射数组转换为适合写入 /proc/[pid]/{uid,gid}_map 的格式
// 格式为：ContainerID HostID Size\n
// 例如：0 1000 1\n2 2000 1\n
// 表示：
//   - 容器内 ID 0 映射到主机 ID 1000，范围为 1
//   - 容器内 ID 2 映射到主机 ID 2000，范围为 1
func formatIDMappings(idMap []syscall.SysProcIDMap) []byte {
	var data []byte
	for _, im := range idMap {
		data = append(data, []byte(strconv.Itoa(im.ContainerID)+" "+strconv.Itoa(im.HostID)+" "+strconv.Itoa(im.Size)+"\n")...)
	}
	return data
}

// writeFile 写入文件内容
// 使用 unix.Open 而不是 ioutil.WriteFile，因为：
// 1. 需要精确控制文件打开标志
// 2. 在处理特殊的 proc 文件系统时更可靠
// 参数：
//   - path: 文件路径
//   - content: 要写入的内容
func writeFile(path string, content []byte) error {
	// 打开文件，设置为读写模式和关闭时执行标志
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	// 写入内容
	if _, err := unix.Write(fd, content); err != nil {
		unix.Close(fd)
		return err
	}
	// 关闭文件
	if err := unix.Close(fd); err != nil {
		return err
	}
	return nil
}
