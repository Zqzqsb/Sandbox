package runner

import (
	"fmt"
	"strconv"
)

// Size 存储对象的字节数，例如内存。
// 最大大小受64位限制
type Size uint64

// String 实现 stringer 接口用于打印
func (s Size) String() string {
	t := uint64(s)
	switch {
	case t < 1<<10:
		return fmt.Sprintf("%d B", t)
	case t < 1<<20:
		return fmt.Sprintf("%.1f KiB", float64(t)/float64(1<<10))
	case t < 1<<30:
		return fmt.Sprintf("%.1f MiB", float64(t)/float64(1<<20))
	default:
		return fmt.Sprintf("%.1f GiB", float64(t)/float64(1<<30))
	}
}

/*
	使用指针接收器时，方法可以直接操作原始对象
	可以修改对象的状态
	不会创建对象的副本，对于大对象更高效
	适用于需要修改对象状态的方法
*/	
// Set 从字符串解析大小值
func (s *Size) Set(str string) error {
	switch str[len(str)-1] {
	case 'b', 'B':
		str = str[:len(str)-1]
	}

	factor := 0
	switch str[len(str)-1] {
	case 'k', 'K':
		factor = 10
		str = str[:len(str)-1]
	case 'm', 'M':
		factor = 20
		str = str[:len(str)-1]
	case 'g', 'G':
		factor = 30
		str = str[:len(str)-1]
	}

	t, err := strconv.Atoi(str)
	if err != nil {
		return err
	}
	*s = Size(t << factor)
	return nil
}


/*
	使用值接收器时，方法会在对象的副本上操作
	方法内部无法修改原始对象的值
	适用于不需要修改对象状态的方法
	每次调用都会创建一个对象的副本，对于大对象可能有性能影响
*/
// Byte 返回字节大小
func (s Size) Byte() uint64 {
	return uint64(s)
}

// KiB 返回 KiB 大小
func (s Size) KiB() uint64 {
	return uint64(s) >> 10
}

// MiB 返回 MiB 大小
func (s Size) MiB() uint64 {
	return uint64(s) >> 20
}

// GiB 返回 GiB 大小
func (s Size) GiB() uint64 {
	return uint64(s) >> 30
}

// TiB 返回 TiB 大小
func (s Size) TiB() uint64 {
	return uint64(s) >> 40
}

// PiB 返回 PiB 大小
func (s Size) PiB() uint64 {
	return uint64(s) >> 50
}

// EiB 返回 EiB 大小
func (s Size) EiB() uint64 {
	return uint64(s) >> 60
}


