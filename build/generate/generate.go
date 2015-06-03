// This file intended only to be called by go generate directive with working directory in cmd/bosun
package main

import (
	"bosun.org/cmd/bosun/web"
	"log"
	"os"
	"os/exec"
)

func main() {
	web.RunTsc()
	web.RunEsc()

	jekylCmd := exec.Command("jekyll", "build", "--source", "../../docs")
	run(jekylCmd)

	escCmd := exec.Command("esc", "-o", "web/docs/docs.go", "-prefix=_site", "-pkg=docs", "_site")
	run(escCmd)

	err := os.RemoveAll("_site")
	if err != nil {
		log.Fatal(err)
	}
}

func run(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}
