// +build ignore,OMIT

package main

import (
	"flag"

	"github.com/khulnasoft-lab/godep/logger"
)

func main() {
	flag.Set("logtostderr", "true")
	logger.Infof("hello, world")
}
