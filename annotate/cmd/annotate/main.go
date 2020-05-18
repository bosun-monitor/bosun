package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"bosun.org/annotate"
)

var rootURLFlag = flag.String("h", "http://bosun/api", "URL of the root of the API")

const flagHostFlagKey = "host"

var (
	startFlag    = flag.String("start", "", fmt.Sprintf("Start time of the annotation. Defaults to now. Format is %v (%v)", timeFormat, timeFormatUsage))
	endFlag      = flag.String("end", "", fmt.Sprintf("End time of the annotation. Defaults to now.  Format is %v (%v)", timeFormat, timeFormatUsage))
	userFlag     = flag.String("user", "", "Username. Defaults to OS username.")
	flagId       = flag.String("id", "", "The GUID of the annotation in order to overwrite an existing annotation.")
	flagHost     = flag.String(flagHostFlagKey, "", "The Host poperty of the annotation. Defaults to the current short lowercase hostname. Set to blank if the host")
	flagOwner    = flag.String("owner", "", "Owner. Defaults to empty")
	flagCategory = flag.String("category", "", "Category. Defaults to empty")
	flagUrl      = flag.String("url", "", "A url, i.e. http://bosun. Defaults to empty")
	flagIn       = flag.Bool("in", false, "use json from standard in to create / update an annotation.")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s [arguments] Message...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *flagIn == true {
		a := procIn()
		sendAnnotation(a)
		os.Exit(0)
	}
	// Message
	remaining := flag.Args()
	if len(remaining) == 0 {
		log.Fatal("A message is required")
	}
	message := strings.Join(remaining, " ")

	// Time
	start := time.Now().UTC()
	end := time.Now().UTC()
	if *startFlag != "" {
		var err error
		start, err = time.Parse(*startFlag, timeFormat)
		if err != nil {
			log.Fatalf("Failed to parse start time, expecting format like %v, got %v", timeFormat, *startFlag)
		}
	}

	// User
	un := *userFlag
	if un == "" {
		// Since os/user requires cgo
		un = os.Getenv(USER_ENV)
		sudo := os.Getenv("SUDO_USER")
		if sudo != "" {
			un = sudo
		}
	}

	// Hostname
	setHostname := false
	for _, arg := range os.Args {
		if arg == "-"+flagHostFlagKey {
			setHostname = true
		}
	}
	var host string
	if !setHostname {
		var err error
		host, err = os.Hostname()
		host = strings.ToLower(host)
		if err == nil {
			host = strings.SplitN(host, ".", 2)[0]
		} else {
			log.Fatal("Failed to go get hostname")
		}
	} else {
		host = *flagHost
	}

	a := annotate.NewAnnotation(*flagId, start, end, un, *flagOwner, "annotate-cli", host, *flagCategory, *flagUrl, message)
	sendAnnotation(a)
}

func procIn() (a annotate.Annotation) {
	d := json.NewDecoder(os.Stdin)
	err := d.Decode(&a)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func sendAnnotation(a annotate.Annotation) {
	c := annotate.NewClient(*rootURLFlag)
	returned, err := c.SendAnnotation(a)
	if err != nil {
		log.Fatal(err)
	}
	raw, err := json.MarshalIndent(returned, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(raw))
}
