package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/howeyc/fsnotify"
)

var (
	command []string
	dirs    []string
)

func main() {
	if len(os.Args) < 3 {
		usage()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)
	bump := make(chan bool)
	go runCmd(bump)

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				//log.Println("event:", ev)
				_ = ev
				bump <- true
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	command = strings.Split(os.Args[len(os.Args)-1], " ")
	dirs = os.Args[1 : len(os.Args)-1]

	for _, dir := range dirs {
		log.Println("watching", dir)
		err = watcher.Watch(dir)
		if err != nil {
			log.Fatal(err)
		}
	}
	<-done
	watcher.Close()
}

func usage() {
	log.Println("usage: watcher dir [dir ...] cmd")
	os.Exit(1)
}

func runCmd(b chan bool) {
	var cmd *exec.Cmd
	next := time.Now()
	for _ = range b {
		if since := time.Since(next); since < 0 {
			//log.Println("too soon, need", -since)
			continue
		}
		if cmd != nil {
			log.Println("stopping")
			if err := cmd.Process.Kill(); err != nil {
				log.Println("failed", err)
			}
			time.Sleep(time.Second * 2)
		}
		cmd = exec.Command(command[0], command[1:]...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Println(1, err)
			continue
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Println(2, err)
			continue
		}
		if err := cmd.Start(); err != nil {
			log.Println(3, err)
			continue
		}
		go func(cmd *exec.Cmd) {
			log.Println("started", cmd.Process.Pid, command)
			if err := cmd.Wait(); err != nil {
				log.Println(4, err)
			}
		}(cmd)
		cmdPrint(stdout, os.Stdout)
		cmdPrint(stderr, os.Stdin)
		next = time.Now().Add(time.Second * 7)
	}
}

func cmdPrint(r io.ReadCloser, f *os.File) {
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			fmt.Fprintln(f, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Println("scan err", err)
		}
	}()
}
