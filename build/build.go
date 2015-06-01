// Simple script to build bosun and scollector. This is not required, but it will properly insert version date and commit
// metadata into the resulting binaries, which `go build` will not do by default.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	// Get current commit SHA
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	sha := strings.TrimSpace(string(output))

	timeStr := time.Now().UTC().Format("20060102150405")

	ldFlags := fmt.Sprintf("-X bosun.org/version.VersionSHA %s -X bosun.org/version.VersionDate %s", sha, timeStr)

	for _, app := range []string{"bosun", "scollector"} {
		cmd = exec.Command("go", "build", "-v", "-ldflags", ldFlags, fmt.Sprintf("bosun.org/cmd/%s", app))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}
