// Gate B: minimal Win7 runtime verification binary.
package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	wd, _ := os.Getwd()
	host, _ := os.Hostname()
	fmt.Println("poc-b OK")
	fmt.Println("go:", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Println("host:", host)
	fmt.Println("cwd:", wd)
	fmt.Println("args:", os.Args)
}
