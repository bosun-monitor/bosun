package main

import (
	"bosun.org/cmd/bosun/web"
	"log"
	"os"
	"os/exec"
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
		cleanedPkg := strings.Replace(line, "/", "_", -1)
		file, err := os.Create("../../docs/pkgs/" + cleanedPkg + ".html")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		pre := `---
layout: goimport
path: `
		if _, err := file.Write([]byte(pre)); err != nil {
			log.Fatal(err)
		}
		if _, err := file.Write([]byte(line)); err != nil {
			log.Fatal(err)
		}
		if _, err := file.Write([]byte("\n---\n")); err != nil {
			log.Fatal(err)
		}
	}
}
