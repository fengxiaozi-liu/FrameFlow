package main

import (
	"fmt"
	"io"
	"os"
)

func printConsoleStatus(w io.Writer, address, logPath string) {
	brand, healthy, reset := "", "", ""
	if os.Getenv("WT_SESSION") != "" {
		brand, healthy, reset = "\x1b[1;38;2;30;190;171m", "\x1b[1;38;2;87;206;139m", "\x1b[0m"
	}
	fmt.Fprintf(w, `
  %sFrameFlow%s                                           LOCAL SERVICE
  ────────────────────────────────────────────────────────────────

  %s●  正在运行%s      本地创作服务已就绪

     访问页面      http://%s
     运行日志      %s

  在浏览器中打开以上地址开始创作。
  可以最小化此窗口；退出请使用页面右上角的「退出」。

`, brand, reset, healthy, reset, address, logPath)
}
