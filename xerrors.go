package xerrors

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
)

/*
希望可以用统一的错误处理接口来处理异常，错误可以分为三个部分来考虑：

1. 错误值
2. 错误处理
3. 错误展示

错误值：
其实错误值只需要扩展两个字段即可，code && message.

Code为暴露出的错误码，由调用方进行进行check，继续业务流程，message为
报错信息，面向内部服务的原则是方便调试，面向外部的原则是不暴露出系统内
敏感信息的前提下方便定位问题。

在Juno或类似主要只是做了数据的存储功能的服务，其实并没有复杂的业务逻辑，
所以这类可以用HTTP STATUS CODE来当做code使用

错误处理：
对于错误处理，由于golang的设计，会导致每一个调用基本都会返回错误，这样
会导致错误处理的重复代码。

例如一下代码，每一个错误都会需要打印日志，有大量的重复代码。

``` golang

func CopyFile(src, dst string) error {
	r, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy %s %s: %v", src, dst, err)
	}
	defer r.Close()

	w, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copy %s %s: %v", src, dst, err)
	}

	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		os.Remove(dst)
		return fmt.Errorf("copy %s %s: %v", src, dst, err)
	}

	if err := w.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy %s %s: %v", src, dst, err)
	}
}
```

官方的2.0改进草案提出了以下的改进方案：

``` golang
func CopyFile(src, dst string) error {
	handle err {
		return fmt.Errorf("copy %s %s: %v", src, dst, err)
	}

	r := check os.Open(src)
	defer r.Close()

	w := check os.Create(dst)
	handle err {
		w.Close()
		os.Remove(dst) // (only if a check fails)
	}

	check io.Copy(w, r)
	check w.Close()
	return nil
}
```
以上方案是先定义handle，然后后续的错误都是用这个handle来处理，只到碰到下一个
handle。

其实核心的思想就是通过预定义统一的handle来复用预处理，那么在代码中也可以这样
干，比如定义一个logErrHandler()来Handle所有需要记录日志的错误。

错误展示：

错误最终是要当做响应返回给调用方的，这个时候就需要同一个错误能够被渲染成不同的
格式，这个功能也是这个模块需要实现的。
*/

func Fail(code int, message string) XError {
	return &xError{
		trace:   nil,
		code:    code,
		message: message}
}

func Failf(code int, format string, args ...interface{}) XError {
	return &xError{
		trace:   nil,
		code:    code,
		message: fmt.Sprintf(format, args...)}
}

func Wrap(err error, errString string) XError {
	if err == nil {
		return nil
	}
	// not create a new error struct
	if re, ok := err.(XError); ok {
		return re.Wrap(err, errString)
	}
	return &xError{
		trace: errors.Wrap(err, errString),
	}
}

func Wrapf(err error, errString string, args ...interface{}) XError {
	if err == nil {
		return nil
	}
	// not create a new error struct
	if re, ok := err.(XError); ok {
		return re.Wrapf(err, errString, args...)
	}
	return &xError{
		trace: errors.Wrapf(err, errString, args...),
	}
}

// only wrap with trace stack, not errString
func WithStack(err error) XError {
	if err == nil {
		return nil
	}
	// not create a new error struct
	if re, ok := err.(XError); ok {
		return re.WithStack(err)
	}
	return &xError{
		trace: errors.WithStack(err),
	}
}

type XError interface {
	Error() string

	Code() int

	Message() string

	Wrap(err error, errString string) XError

	Wrapf(err error, errString string, args ...interface{}) XError

	WithStack(err error) XError

	GetError() error
}

type xError struct {
	trace error

	code    int
	message string
}

func (xe *xError) Error() string {
	if xe.trace == nil {
		return ""
	}
	return xe.trace.Error()
}

func (xe *xError) Code() int {
	return xe.code
}

func (xe *xError) Message() string {
	return xe.message
}

// Design of merge two XError:
// 1. Save two trace of every XError to trace message.
// 2. Save the formatted code and message from wrapped XError to trace message.
// 3. Save the Wrap XError code and message as the new XError.
func (xe *xError) Wrap(err error, errString string) XError {

	if re, ok := err.(XError); ok {
		var newTrace error
		// When xe equals with re, means that `Wrap` called by module api, but not
		// interface `XError` api.

		// **So it's self wrap self**

		// Then, we do not need wrap it again
		if re == xe {
			newTrace = re.GetError()
		} else {
			newTrace = xe.WithStack(re.GetError())
		}
		xe.trace = errors.WithMessage(newTrace, errString)
	} else {
		xe.trace = errors.Wrap(err, errString)
	}
	return xe
}

func (xe *xError) Wrapf(err error, errString string, args ...interface{}) XError {
	if re, ok := err.(XError); ok {
		var newTrace error
		// same as above
		if re == xe {
			newTrace = re.GetError()
		} else {
			newTrace = xe.WithStack(re.GetError())
		}
		xe.trace = errors.WithMessage(newTrace, fmt.Sprintf(errString, args...))
	} else {
		xe.trace = errors.Wrapf(err, errString, args...)
	}
	return xe
}

func (xe *xError) WithStack(err error) XError {
	xe.trace = errors.WithStack(err)
	if re, ok := err.(XError); ok {
		// same as above
		if re == xe {
			return re
		}
		var rawMessage= ``
		rawMessage = fmt.Sprintf(`<Error %d>: %s`, re.Code(), re.Message())
		// save raw exception message
		if len(rawMessage) != 0 {
			xe.trace = errors.WithMessage(xe.trace, rawMessage)
		}
	}
	return xe
}

func (xe *xError) GetError() error {
	return xe.trace
}

func (xe *xError) Cause() error {
	causer, ok := xe.trace.(causer)
	if !ok {
		return xe.trace
	}
	return causer.Cause()
}

func (xe *xError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", xe.trace)
			// follows code will redirect invoke method by reflect
			//val := reflect.ValueOf(xe.trace)
			//params := make([]reflect.Value,2)
			//params[0] = reflect.ValueOf(s)
			//params[1] = reflect.ValueOf(verb)
			//val.MethodByName("Format").Call(params)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, xe.Error())
	case 'q':
		fmt.Fprintf(s, "%q", xe.Error())
	}
}

type causer interface {
	Cause() error
}

// If the error does not implement Cause, the original error will

// be returned. If the error is nil, nil will be returned without further
// investigation.
func Cause(err error) error {

	for err != nil {
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}
	return err
}
