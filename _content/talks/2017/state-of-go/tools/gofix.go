// +build ignore,OMIT

package main

import "github.com/khulnasoft-lab/godep/net/context" // HL

func main() {
	ctx := context.Background()
	doSomething(ctx)
}

func doSomething(ctx context.Context) {
	// doing something
}
