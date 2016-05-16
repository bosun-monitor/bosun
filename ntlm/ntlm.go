package ntlm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync/atomic"
)

// DoNTLMRequest Perform a request using NTLM authentication
func DoNTLMRequest(httpClient *http.Client, request *http.Request) (*http.Response, error) {

	handshakeReq, err := cloneRequest(request)
	if err != nil {
		return nil, err
	}

	res, err := httpClient.Do(handshakeReq)
	if err != nil && res == nil {
		return nil, err
	}

	//If the status is 401 then we need to re-authenticate, otherwise it was successful
	if res.StatusCode == 401 {

		auth, authOk := getDefaultCredentialsAuth()
		if authOk {
			negotiateMessageBytes, err := auth.GetNegotiateBytes()
			if err != nil {
				return nil, err
			}
			defer auth.ReleaseContext()

			negotiateReq, err := cloneRequest(request)
			if err != nil {
				return nil, err
			}

			challengeMessage, err := sendNegotiateRequest(httpClient, negotiateReq, negotiateMessageBytes)
			if err != nil {
				return nil, err
			}

			challengeReq, err := cloneRequest(request)
			if err != nil {
				return nil, err
			}

			responseBytes, err := auth.GetResponseBytes(challengeMessage)

			res, err := sendChallengeRequest(httpClient, challengeReq, responseBytes)
			if err != nil {
				return nil, err
			}

			return res, nil
		}
	}

	return res, nil
}

func sendNegotiateRequest(httpClient *http.Client, request *http.Request, negotiateMessageBytes []byte) ([]byte, error) {
	negotiateMsg := base64.StdEncoding.EncodeToString(negotiateMessageBytes)

	request.Header.Add("Authorization", "NTLM "+negotiateMsg)
	res, err := httpClient.Do(request)

	if res == nil && err != nil {
		return nil, err
	}

	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()

	ret, err := parseChallengeResponse(res)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func sendChallengeRequest(httpClient *http.Client, request *http.Request, challengeBytes []byte) (*http.Response, error) {
	authMsg := base64.StdEncoding.EncodeToString(challengeBytes)
	request.Header.Add("Authorization", "NTLM "+authMsg)
	return httpClient.Do(request)
}

func parseChallengeResponse(response *http.Response) ([]byte, error) {
	header := response.Header.Get("Www-Authenticate")
	if len(header) < 6 {
		return nil, fmt.Errorf("Invalid NTLM challenge response: %q", header)
	}

	//parse out the "NTLM " at the beginning of the response
	challenge := header[5:]
	val, err := base64.StdEncoding.DecodeString(challenge)

	if err != nil {
		return nil, err
	}
	return []byte(val), nil
}

func cloneRequest(request *http.Request) (*http.Request, error) {
	cloneReqBody, err := cloneRequestBody(request)
	if err != nil {
		return nil, err
	}

	clonedReq, err := http.NewRequest(request.Method, request.URL.String(), cloneReqBody)
	if err != nil {
		return nil, err
	}

	for k := range request.Header {
		clonedReq.Header.Add(k, request.Header.Get(k))
	}

	clonedReq.TransferEncoding = request.TransferEncoding
	clonedReq.ContentLength = request.ContentLength

	return clonedReq, nil
}

func cloneRequestBody(req *http.Request) (io.ReadCloser, error) {
	if req.Body == nil {
		return nil, nil
	}

	var cb *cloneableBody
	var err error
	isCloneableBody := true

	// check to see if the request body is already a cloneableBody
	body := req.Body
	if existingCb, ok := body.(*cloneableBody); ok {
		isCloneableBody = false
		cb, err = existingCb.CloneBody()
	} else {
		cb, err = newCloneableBody(req.Body, 0)
	}

	if err != nil {
		return nil, err
	}

	if isCloneableBody {
		cb2, err := cb.CloneBody()
		if err != nil {
			return nil, err
		}

		req.Body = cb2
	}

	return cb, nil
}

type cloneableBody struct {
	bytes  []byte    // in-memory buffer of body
	file   *os.File  // file buffer of in-memory overflow
	reader io.Reader // internal reader for Read()
	closed bool      // tracks whether body is closed
	dup    *dupTracker
}

func newCloneableBody(r io.Reader, limit int64) (*cloneableBody, error) {
	if limit < 1 {
		limit = 1048576 // default
	}

	b := &cloneableBody{}
	buf := &bytes.Buffer{}
	w, err := io.CopyN(buf, r, limit)
	if err != nil && err != io.EOF {
		return nil, err
	}

	b.bytes = buf.Bytes()
	byReader := bytes.NewBuffer(b.bytes)

	if w >= limit {
		tmp, err := ioutil.TempFile("", "git-lfs-clone-reader")
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(tmp, r)
		tmp.Close()
		if err != nil {
			os.RemoveAll(tmp.Name())
			return nil, err
		}

		f, err := os.Open(tmp.Name())
		if err != nil {
			os.RemoveAll(tmp.Name())
			return nil, err
		}

		dups := int32(0)
		b.dup = &dupTracker{name: f.Name(), dups: &dups}
		b.file = f
		b.reader = io.MultiReader(byReader, b.file)
	} else {
		// no file, so set the reader to just the in-memory buffer
		b.reader = byReader
	}

	return b, nil
}

func (b *cloneableBody) Read(p []byte) (int, error) {
	if b.closed {
		return 0, io.EOF
	}
	return b.reader.Read(p)
}

func (b *cloneableBody) Close() error {
	if !b.closed {
		b.closed = true
		if b.file == nil {
			return nil
		}

		b.file.Close()
		b.dup.Rm()
	}
	return nil
}

func (b *cloneableBody) CloneBody() (*cloneableBody, error) {
	if b.closed {
		return &cloneableBody{closed: true}, nil
	}

	b2 := &cloneableBody{bytes: b.bytes}

	if b.file == nil {
		b2.reader = bytes.NewBuffer(b.bytes)
	} else {
		f, err := os.Open(b.file.Name())
		if err != nil {
			return nil, err
		}
		b2.file = f
		b2.reader = io.MultiReader(bytes.NewBuffer(b.bytes), b2.file)
		b2.dup = b.dup
		b.dup.Add()
	}

	return b2, nil
}

type dupTracker struct {
	name string
	dups *int32
}

func (t *dupTracker) Add() {
	atomic.AddInt32(t.dups, 1)
}

func (t *dupTracker) Rm() {
	newval := atomic.AddInt32(t.dups, -1)
	if newval < 0 {
		os.RemoveAll(t.name)
	}
}
