package httpcontrol_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"bosun.org/_third_party/github.com/facebookgo/httpcontrol"
)

func Example() {
	// The provided Transport can be used via the HTTP client or used directly
	// via RoundTrip.
	client := &http.Client{
		Transport: &httpcontrol.Transport{
			RequestTimeout: time.Minute,
			MaxTries:       3,
		},
	}

	res, err := client.Get("http://graph.facebook.com/DoctorWho?fields=name")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer res.Body.Close()

	// Just outputting the response for example purposes.
	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Output: {"name":"Doctor Who","id":"127031120644257"}
}
