package main

import (
	"fmt"
	// "gitlab.devops.rdrk.com.cn/quant/yd4go"
)

var (
	version, goVersion, gitVersion, buildTime string

	// api *yd4go.YdApi
)

func init() {
	fmt.Printf(
		"[LIB emc4go] %s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	)
}
