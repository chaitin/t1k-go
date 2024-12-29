package misc

import (
	"fmt"
	"runtime"
)

// wrapError 结构体用于包装错误信息
type wrapError struct {
	message string  // 错误消息
	next    error   // 原始错误
	pc      uintptr // 程序计数器，用于存储调用栈信息
	file    string  // 源文件名
	line    int     // 行号
}

// Unwrap 方法实现错误链的展开
func (e *wrapError) Unwrap() error {
	return e.next
}

// Error 方法返回格式化的错误信息
func (e *wrapError) Error() string {
	if e.next == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %v", e.message, e.next)
}

// Format 方法实现自定义错误格式化
func (e *wrapError) Format(f fmt.State, c rune) {
	// 根据不同的格式化符号选择不同的输出
	switch c {
	case 'v':
		if f.Flag('+') {
			// 详细模式，包含堆栈信息
			fmt.Fprintf(f, "%s\n\t%s:%d", e.message, e.file, e.line)
			if e.next != nil {
				fmt.Fprintf(f, "\n%+v", e.next)
			}
			return
		}
		fallthrough
	case 's':
		// 简单模式，只显示错误信息
		fmt.Fprint(f, e.Error())
	}
}

// wrap 是一个内部函数，用于包装错误并记录调用位置
func wrap(err error, message string, skip int) error {
	if err == nil {
		return nil
	}
	// 获取调用者的文件和行号信息
	pc, file, line, _ := runtime.Caller(skip)
	return &wrapError{
		message: message,
		next:    err,
		pc:      pc,
		file:    file,
		line:    line,
	}
}

// ErrorWrap 包装错误并添加额外信息
func ErrorWrap(err error, message string) error {
	return wrap(err, message, 2)
}

// ErrorWrapf 使用格式化字符串包装错误
func ErrorWrapf(err error, message string, args ...interface{}) error {
	return wrap(err, fmt.Sprintf(message, args...), 2)
}
