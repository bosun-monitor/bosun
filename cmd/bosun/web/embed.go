package web

import (
	"bufio"
	"bytes"
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
	run("esc", "-modtime", "0", "-o", "web/static.go", "-pkg", "web", "-prefix", "web/static", "web/static")
}

// Run tsc to compile all ts files into bosun.js
func RunTsc() {
	base := filepath.Join("web", "static", "js")
	tmp := filepath.Join(base, "bosun-new.js")
	dst := filepath.Join(base, "bosun.js")
	args := []string{
		"--out", tmp,
	}
	matches, _ := filepath.Glob(filepath.Join(base, "*.ts"))
	sort.Strings(matches)
	args = append(args, matches...)
	run("tsc", args...)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		overwriteFile(tmp, dst)
	} else {
		if deepCompareDifferent(tmp, dst) {
			overwriteFile(tmp, dst)
		} else {
			err := os.Remove(tmp)
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
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

func deepCompareDifferent(file1, file2 string) bool {
	sf, err := os.Open(file1)
	if err != nil {
		log.Fatal(err)
	}
	df, err := os.Open(file2)
	if err != nil {
		log.Fatal(err)
	}
	defer sf.Close()
	defer df.Close()
	sscan := bufio.NewScanner(sf)
	dscan := bufio.NewScanner(df)
	for sscan.Scan() {
		dscan.Scan()
		if !bytes.Equal(sscan.Bytes(), dscan.Bytes()) {
			return true
		}
	}
	return false
}

func overwriteFile(filesrc, filedst string) {
	err := os.Rename(filesrc, filedst)
	if err != nil {
		log.Println(err)
		return
	}
}
