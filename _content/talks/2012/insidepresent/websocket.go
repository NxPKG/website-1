// +build ignore,OMIT

package main

import (
	"fmt"
	"github.com/khulnasoft-lab/godep/net/websocket"
	"net/http"
)

func main() {
	http.Handle("/", websocket.Handler(handler))
	http.ListenAndServe("localhost:4000", nil)
}

func handler(c *websocket.Conn) {
	var s string
	fmt.Fscan(c, &s)
	fmt.Println("Received:", s)
	fmt.Fprint(c, "How do you do?")
}
