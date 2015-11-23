package main

import (
	"bosun.org/cmd/bosun/web"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	generateDocImports()
	web.RunTsc()
	web.RunEsc()
}

func generateDocImports() {
	// generate github pages so go-get redirects work.
	cmd := exec.Command("go", "list", "bosun.org...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {

		if line == "" || !strings.HasPrefix(line, "bosun.org/") {
			continue
		}
		line = line[10:]
		if strings.HasPrefix(line, "docs") || strings.HasPrefix(line, "build") {
			continue
		}
		fileName := filepath.Join("../../docs", line, "index.html")

		err = os.MkdirAll(filepath.Dir(fileName), 0777)
		if err != nil {
			log.Fatal(err)

		}

		file, err := os.Create(fileName)
		if err != nil {
			log.Fatal(err)

		}
		text := `---
layout: goimport
path: ***
---
			`
		text = strings.Replace(text, "***", line, -1)
		if _, err := file.Write([]byte(text)); err != nil {
			log.Fatal(err)
		}
		file.Close()
	}
}
