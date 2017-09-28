package web

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/mjibson/esc/embed"
)

// Functions to run tsc to compile typescript and esc to embed static assests.
// Assume that working path is set to `cmd/bosun`

// Run esc to embed entire static directory into static.go
func RunEsc() {
	cfg := &embed.Config{
		ModTime:    "0",
		OutputFile: "web/static.go",
		Package:    "web",
		Prefix:     "web/static",
		Ignore:     `.*\.ts`,
		Files:      []string{"web/static"},
	}
	embed.Run(cfg)
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
	log.Println("running", name, arg)
	c := exec.Command(name, arg...)
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	if err := c.Run(); err != nil {
		log.Printf("run error: %v: %v", name, err)
	}
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
