// Simple script to build bosun and scollector. This is not required, but it will properly insert version date and commit
// metadata into the resulting binaries, which `go build` will not do by default.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	shaFlag         = flag.String("sha", "", "SHA to embed.")
	buildBosun      = flag.Bool("bosun", false, "Only build Bosun.")
	buildTsdb       = flag.Bool("tsdbrelay", false, "Only build tsdbrelay")
	buildScollector = flag.Bool("scollector", false, "Only build scollector.")
	output          = flag.String("output", "", "Output directory; defaults to $GOPATH/bin.")

	allProgs = []string{"bosun", "scollector", "tsdbrelay"}
)

func main() {
	flag.Parse()
	// Get current commit SHA
	sha := *shaFlag
	if sha == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Stderr = os.Stderr
		output, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		sha = strings.TrimSpace(string(output))
	}

	timeStr := time.Now().UTC().Format("20060102150405")
	ldFlags := fmt.Sprintf("-X bosun.org/_version.VersionSHA=%s -X bosun.org/_version.VersionDate=%s", sha, timeStr)

	progs := allProgs
	if *buildBosun {
		progs = []string{"bosun"}
	} else if *buildScollector {
		progs = []string{"scollector"}
	} else if *buildTsdb {
		progs = []string{"tsdbrelay"}
	}
	for _, app := range progs {
		fmt.Println("building", app)
		var args []string
		if *output != "" {
			args = append(args, "build", "-o", filepath.Join(*output, app))
		} else {
			args = append(args, "install")
		}
		args = append(args, "-ldflags", ldFlags, fmt.Sprintf("bosun.org/cmd/%s", app))
		fmt.Println("go", strings.Join(args, " "))
		cmd := exec.Command("go", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}
