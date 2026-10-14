package store

import "errors"

// ErrNotFound 目标记录不存在或归属校验失败（对外统一视为不存在）。
var ErrNotFound = errors.New("not found")

// ErrDupUser 用户名已存在。
var ErrDupUser = errors.New("用户名已被占用")
