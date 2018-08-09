package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"bytes"

	"bosun.org/util"
)

var (
	flagHost     = flag.String("h", "bosun", "Hostname of your bosun server, defaults to bosun.")
	flagUser     = flag.String("u", "", "Username, defaults to the username returned from the OS.")
	flagDuration = flag.String("d", "30m", "A duration to silence this host for. Defaults to 30m and the format is defined at https://golang.org/pkg/time/#ParseDuration")
	flagTags     = flag.String("t", "", "OpenTSDB tags to be silenced in the format of tagKey=value,tagKey=value. Defaults to host=<hostname>, use -t= to use empty tag set.")
	flagAlert    = flag.String("a", "", "Name of the alert to silence, defaults to empty which means all alerts.")
	flagMessage  = flag.String("m", "", "Reason for the silence, defaults to an empty string.")
	flagForget   = flag.String("f", "", "Set to 'true' to forget anything that goes unknown during the silence. Used when decommissioning something.")
)

func main() {
	flag.Parse()
	un := *flagUser
	if un == "" {
		// Use os.Getenv because os/user and user.Current() requires cgo
		switch runtime.GOOS {
		case "darwin", "linux":
			un = os.Getenv("USER")
		case "windows":
			un = os.Getenv("USERNAME")
		default:
			un = "UNKNOWN"
		}
		sudo := os.Getenv("SUDO_USER")
		if sudo != "" {
			un = sudo
		}
	}
	if *flagTags == "" {
		flagTagsIsPresent := false
		for _, arg := range os.Args {
			if arg == "-t=" {
				flagTagsIsPresent = true
			}
		}
		if !flagTagsIsPresent {
			*flagTags = "host=" + util.Hostname
		}
	}
	now := time.Now().UTC()
	d, err := time.ParseDuration(*flagDuration)
	if err != nil {
		log.Fatal(err)
	}
	end := now.Add(d)
	if *flagForget != "" {
		*flagForget = "true"
	}
	s := struct {
		User    string `json:"user"`
		Start   string `json:"start"`
		End     string `json:"end"`
		Tags    string `json:"tags"`
		Alert   string `json:"alert"`
		Message string `json:"message"`
		Confirm string `json:"confirm"`
		Forget  string `json:"forget"`
	}{
		un,
		now.Format("2006-01-02 15:04:05 MST"),
		end.Format("2006-01-02 15:04:05 MST"),
		*flagTags,
		*flagAlert,
		*flagMessage,
		"confirm",
		*flagForget,
	}
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatal(err, s)
	}
	resp, err := http.Post("https://"+*flagHost+"/api/silence/set", "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Fatal(err, s)
	}
	c, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("%s", c)
	}
	if s.Alert == "" {
		s.Alert = "None"
	}
	if s.Tags == "" {
		s.Tags = "None"
	}
	if s.Message == "" {
		s.Message = "None"
	}
	fmt.Printf("Created silence: Start: %s, End: %s, Tags: %s, Alert: %s, Message: %s\n",
		s.Start, s.End, s.Tags, s.Alert, s.Message)
}
