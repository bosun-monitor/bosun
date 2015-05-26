package web

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// Functions to run tsc to compile typescript and esc to embed static assests.
// Assume that working path is set to `cmd/bosun`

// Run esc to embed entire static directory into static.go
func RunEsc() {
	run("esc", "-o", "web/static.go", "-pkg", "web", "-prefix", "web/static", "web/static")
}

// Run tsc to compile all ts files into bosun.js
func RunTsc() {
	base := filepath.Join("web", "static", "js")
	args := []string{
		"--out", filepath.Join(base, "bosun.js"),
	}
	matches, _ := filepath.Glob(filepath.Join(base, "*.ts"))
	sort.Strings(matches)
	args = append(args, matches...)
	run("tsc", args...)
}

func run(name string, arg ...string) {
	log.Println("running", name)
	c := exec.Command(name, arg...)
	stdout, err := c.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
	go func() { io.Copy(os.Stdout, stdout) }()
	go func() { io.Copy(os.Stderr, stderr) }()
	if err := c.Wait(); err != nil {
		log.Printf("run error: %v: %v", name, err)
	}
	log.Println("run complete:", name)
}
