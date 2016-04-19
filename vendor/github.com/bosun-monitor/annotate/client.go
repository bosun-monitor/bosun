package annotate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type Client struct {
	apiRoot string
	client  *http.Client
}

func NewClient(apiRoot string) Client {
	return Client{
		apiRoot: apiRoot,
		client:  &http.Client{},
	}
}

// SendAnnotation sends a annotation to an annotations server
// apiRoot should be "http://foo/api" where the annotation routes
// would be available at http://foo/api/annotation...
func (c *Client) SendAnnotation(a Annotation) (Annotation, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return a, err
	}

	res, err := c.client.Post(c.apiRoot+"/annotation", "application/json", bytes.NewReader(b))
	if err != nil {
		return a, err
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&a)
	return a, err
}

// GetAnnotation gets an annotation by ID, and will return
// nil without an error if the annotation does not exist
func (c *Client) GetAnnotation(id string) (*Annotation, error) {
	a := &Annotation{}
	res, err := c.client.Get(c.apiRoot + "/annotation/" + id)
	if res != nil && res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&a)
	return a, err
}

func (c *Client) GetAnnotations(start, end *time.Time, source, host, creationUser, owner, category, url, message string) (Annotations, error) {
	a := Annotations{}
	req, err := http.NewRequest("GET", c.apiRoot+"/annotation/query", nil)
	if err != nil {
		return a, nil
	}
	q := req.URL.Query()
	if start != nil {
		q.Add(StartDate, start.Format(time.RFC3339))
	}
	if end != nil {
		q.Add(EndDate, end.Format(time.RFC3339))
	}
	if source != "" {
		q.Add(Source, source)
	}
	if host != "" {
		q.Add(Host, host)
	}
	if creationUser != "" {
		q.Add(CreationUser, creationUser)
	}
	if owner != "" {
		q.Add(Owner, owner)
	}
	if category != "" {
		q.Add(Category, category)
	}
	if url != "" {
		q.Add(Url, url)
	}
	if message != "" {
		q.Add(Message, message)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return a, err
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&a)
	return a, err
}
