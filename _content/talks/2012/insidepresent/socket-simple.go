// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore,OMIT

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/khulnasoft-lab/godep/net/websocket"
)

const socketPresent = true

func HandleSocket(path string) {
	http.Handle(path, websocket.Handler(socketHandler))
}

const msgLimit = 1000 // max number of messages to send per session

var uniq = make(chan int) // a source of numbers for naming temporary files

func init() {
	go func() {
		for i := 0; ; i++ {
			uniq <- i
		}
	}()
}

// Message is the wire format for the websocket connection to the browser.
// It is used for both sending output messages and receiving commands, as
// distinguished by the Kind field.
type Message struct {
	Id   string // client-provided unique id for the process
	Kind string // in: "run", "kill" out: "stdout", "stderr", "end"
	Body string
}

// socketHandler handles the websocket connection for a given present session.
// It handles transcoding Messages to and from JSON format, and starting
// and killing Processes.
func socketHandler(c *websocket.Conn) {
	in, out := make(chan *Message), make(chan *Message)
	errc := make(chan error, 1)

	// Decode messages from client and send to the in channel.
	go func() {
		dec := json.NewDecoder(c)
		for {
			var m Message
			if err := dec.Decode(&m); err != nil {
				errc <- err
				return
			}
			in <- &m
		}
	}()

	// Receive messages from the out channel and encode to the client.
	go func() {
		enc := json.NewEncoder(c)
		for m := range out {
			if err := enc.Encode(m); err != nil {
				errc <- err
				return
			}
		}
	}()
	// END OMIT

	// Start and kill Processes and handle errors.
	proc := make(map[string]*Process)
	for {
		select {
		case m := <-in:
			switch m.Kind {
			case "run":
				proc[m.Id].Kill()
				proc[m.Id] = StartProcess(m.Id, m.Body, out)
			case "kill":
				proc[m.Id].Kill()
			}
		case err := <-errc:
			// A encode or decode has failed; bail.
			log.Println(err)
			// Shut down any running processes.
			for _, p := range proc {
				p.Kill()
			}
			return
		}
	}
}

// Process represents a running process.
type Process struct {
	id   string
	out  chan<- *Message
	done chan struct{} // closed when wait completes
	run  *exec.Cmd
}

// StartProcess builds and runs the given program, sending its output
// and end event as Messages on the provided channel.
func StartProcess(id, body string, out chan<- *Message) *Process {
	p := &Process{
		id:   id,
		out:  out,
		done: make(chan struct{}),
	}
	cmd, err := p.start(body)
	if err != nil {
		p.end(err)
		return nil
	}
	p.run = cmd
	go p.wait(cmd)
	return p
}

// Kill stops the process if it is running and waits for it to exit.
func (p *Process) Kill() {
	if p == nil {
		return
	}
	if p.run != nil {
		p.run.Process.Kill()
	}
	<-p.done
}

// start builds and starts the given program, sending its output to p.out,
// and returns the associated *exec.Cmd.
func (p *Process) start(body string) (*exec.Cmd, error) {
	// x is the base name for .go and executable files
	x := filepath.Join(tmpdir, "compile"+strconv.Itoa(<-uniq))
	src := x + ".go"
	bin := x
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	// write body to x.go
	defer os.Remove(src)
	if err := ioutil.WriteFile(src, []byte(body), 0666); err != nil {
		return nil, err
	}
	// END OMIT

	// build x.go, creating x
	dir, file := filepath.Split(src)
	err := p.cmd(dir, "go", "build", "-o", bin, file).Run()
	defer os.Remove(bin)
	if err != nil {
		return nil, err
	}

	// run x
	cmd := p.cmd("", bin)
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// wait waits for the running process to complete and returns its error state.
func (p *Process) wait(cmd *exec.Cmd) {
	defer close(p.done)
	p.end(cmd.Wait())
}

// end sends an "end" message to the client, containing the process id and the
// given error value.
func (p *Process) end(err error) {
	m := &Message{Id: p.id, Kind: "end"}
	if err != nil {
		m.Body = err.Error()
	}
	p.out <- m
}

// cmd builds an *exec.Cmd that writes its standard output and error to the
// Process' output channel.
func (p *Process) cmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = &messageWriter{p.id, "stdout", p.out}
	cmd.Stderr = &messageWriter{p.id, "stderr", p.out}
	return cmd
}

// messageWriter is an io.Writer that converts all writes to Message sends on
// the out channel with the specified id and kind.
type messageWriter struct {
	id, kind string
	out      chan<- *Message
}

func (w *messageWriter) Write(b []byte) (n int, err error) {
	w.out <- &Message{Id: w.id, Kind: w.kind, Body: string(b)}
	return len(b), nil
}

// END OMIT

var tmpdir string

func init() {
	// find real path to temporary directory
	var err error
	tmpdir, err = filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		log.Fatal(err)
	}
}
