
## xERRORS

------------------------

### 名词解释

- 错误: 由系统抛出的errors
- 异常：xerrors.New创建的错误

### Usage

1.在认为可能出现**错误**的地方Wrap，收集需要的错误栈信息(Wrap会收集错误栈，多次调用不会生成多个错误栈)
```go
	_, err := os.Open(src)
	if err != nil {
		return xerrors.Wrap(err, "open file failed")
	}
```
或者不需要记录额外的日志信息的时候:
```go
	_, err := os.Open(src)
	if err != nil {
		return xerrors.WithStack(err)
	}
```

2.在尽可能早的时候，确认了业务逻辑出现**异常**，需要抛出**异常**
```go
	_, err := os.Open(src)
	if err != nil {
		return xerrors.Fail(http.StatusNotFound, "file not exits")
	}
	
```

3.在不清楚是否底层已经抛出业务逻辑的**异常**，而且想要再抛出一个新的**异常**时
```go
	_, err := service.find_user(userID)
	if err != nil {
		return xerrors.Failf(http.StatusNotFound, "user not find: %s", userID).WithStack(err)
	}
	
```

如果底层存在异常，则旧的异常的Code和Message会被格式化成string后，作为debug info存入新的
**异常**的错误栈内

4.在需要处理**异常**的时候，使用Cause来获取原始的**错误**，再进行handle
```go
	_, err := os.Open(src)
	if err != nil {
        if xerrors.Cause(err) == FileNotExists {
            return xerrors.Fail(http.StatusNotFound, "file not exits")
        }
        return err
	}

```

5.在需要把**异常**作为响应返回给调用方的时候。
```go
    xe, ok := err.(XError)
    if !ok {
        xe = xerrors.Fail(http.StatusInternalServerError, "Internal Server Error").WithStack(err)
    }
    
    if xe.Code() == 0 && xe.Message() == "" {
        xe = xerrors.Fail(http.StatusInternalServerError, "Internal Server Error").WithStack(err)
    }

    xe.Code(), xe.Message()
```

## Todo:
- [] [每次Wrap都会产生新的错误栈](https://github.com/pkg/errors/issues/75)