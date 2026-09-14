package web

import "embed"

// Files 包含生产环境的 Vue 构建产物。Windows 构建脚本会在编译独立可执行文件前刷新此目录。
//
//go:embed dist/*
var Files embed.FS
