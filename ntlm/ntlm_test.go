package ntlm

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestNtlmCloneRequest(t *testing.T) {
	req1, _ := http.NewRequest("Method", "url", nil)
	cloneOfReq1, err := cloneRequest(req1)

	if err != nil {
		t.Fatalf("Unexpected exception!")
	}

	assertRequestsEqual(t, req1, cloneOfReq1)

	req2, _ := http.NewRequest("Method", "url", bytes.NewReader([]byte("Some data")))
	cloneOfReq2, err := cloneRequest(req2)

	if err != nil {
		t.Fatalf("Unexpected exception!")
	}

	assertRequestsEqual(t, req2, cloneOfReq2)
}

func assertRequestsEqual(t *testing.T, req1 *http.Request, req2 *http.Request) {

	if req1.Method != req2.Method {
		t.Fatalf("HTTP request methods are not equal")
	}

	for k, v := range req1.Header {
		if !reflect.DeepEqual(v, req2.Header[k]) {
			t.Fatalf("Headers are not equal!")
		}
	}

	if req1.Body == nil {
		if req2.Body != nil {
			t.Fatalf("Request body should be nil")
		}
	} else {
		bytes1, _ := ioutil.ReadAll(req1.Body)
		bytes2, _ := ioutil.ReadAll(req2.Body)
		if bytes.Compare(bytes1, bytes2) != 0 {
			t.Fatalf("Bytes of request bodies should be the same")
		}
	}
}

func TestNtlmHeaderParseValid(t *testing.T) {
	res := http.Response{}
	res.Header = make(map[string][]string)
	res.Header.Add("Www-Authenticate", "NTLM "+base64.StdEncoding.EncodeToString([]byte("Some data")))
	bytes, err := parseChallengeResponse(&res)

	if err != nil {
		t.Fatalf("Unexpected exception!")
	}

	// Check NTLM has been stripped from response
	if strings.HasPrefix(string(bytes), "NTLM") {
		t.Fatalf("Response contains NTLM prefix!")
	}
}

func TestNtlmHeaderParseInvalidLength(t *testing.T) {
	res := http.Response{}
	res.Header = make(map[string][]string)
	res.Header.Add("Www-Authenticate", "NTL")
	ret, err := parseChallengeResponse(&res)
	if ret != nil {
		t.Errorf("Unexpected challenge response: %v", ret)
	}

	if err == nil {
		t.Errorf("Expected error, got none!")
	}
}

func TestNtlmHeaderParseInvalid(t *testing.T) {
	res := http.Response{}
	res.Header = make(map[string][]string)
	res.Header.Add("Www-Authenticate", base64.StdEncoding.EncodeToString([]byte("NTLM I am a moose")))
	_, err := parseChallengeResponse(&res)

	if err == nil {
		t.Fatalf("Expected error, got none!")
	}
}

func TestCloneSmallBody(t *testing.T) {
	cloneable1, err := newCloneableBody(strings.NewReader("abc"), 5)
	if err != nil {
		t.Fatal(err)
	}

	cloneable2, err := cloneable1.CloneBody()
	if err != nil {
		t.Fatal(err)
	}

	assertCloneableBody(t, cloneable2, "abc", "abc")
	assertCloneableBody(t, cloneable1, "abc", "abc")
}

func TestCloneBigBody(t *testing.T) {
	cloneable1, err := newCloneableBody(strings.NewReader("abc"), 2)
	if err != nil {
		t.Fatal(err)
	}

	cloneable2, err := cloneable1.CloneBody()
	if err != nil {
		t.Fatal(err)
	}

	assertCloneableBody(t, cloneable2, "abc", "ab")
	assertCloneableBody(t, cloneable1, "abc", "ab")
}

func assertCloneableBody(t *testing.T, cloneable *cloneableBody, expectedBody, expectedBuffer string) {
	buffer := string(cloneable.bytes)
	if buffer != expectedBuffer {
		t.Errorf("Expected buffer %q, got %q", expectedBody, buffer)
	}

	if cloneable.closed {
		t.Errorf("already closed?")
	}

	by, err := ioutil.ReadAll(cloneable)
	if err != nil {
		t.Fatal(err)
	}

	if err := cloneable.Close(); err != nil {
		t.Errorf("Error closing: %v", err)
	}

	actual := string(by)
	if actual != expectedBody {
		t.Errorf("Expected to read %q, got %q", expectedBody, actual)
	}
}
