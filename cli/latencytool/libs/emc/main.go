package main

import "fmt"

var (
	version, goVersion, gitVersion, buildTime string
)

func init() {
	fmt.Printf(
		"[LIB emc4go] %s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	)
}
