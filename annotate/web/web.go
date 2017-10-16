package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bosun.org/annotate"
	"bosun.org/annotate/backend"

	"github.com/gorilla/mux"
	"github.com/twinj/uuid"
)

// esc -o static.go -pkg web static

//AddRoutes will add annotate routes to the given router, using the specified prefix
func AddRoutes(router *mux.Router, prefix string, b []backend.Backend, enableUI, useLocalAssets bool) error {
	return AddRoutesWithMiddleware(router, prefix, b, enableUI, useLocalAssets, nil, nil)
}

func noopMiddleware(h http.HandlerFunc) http.Handler { return h }

//AddRoutesWithMiddleware will add annotate routes to the given router, using the specified prefix. It accepts two middleware functions that will be applied to each route,
//depending on whether they are a "read" operation, or a "write" operation
func AddRoutesWithMiddleware(router *mux.Router, prefix string, b []backend.Backend, enableUI, useLocalAssets bool, readMiddleware, modifyMiddleware func(http.HandlerFunc) http.Handler) error {
	if readMiddleware == nil {
		readMiddleware = noopMiddleware
	}
	if modifyMiddleware == nil {
		modifyMiddleware = noopMiddleware
	}
	backends = b
	router.Handle(prefix+"/annotation", modifyMiddleware(InsertAnnotation)).Methods("POST", "PUT")
	router.Handle(prefix+"/annotation/query", readMiddleware(GetAnnotations)).Methods("GET")
	router.Handle(prefix+"/annotation/{id}", readMiddleware(GetAnnotation)).Methods("GET")
	router.Handle(prefix+"/annotation/{id}", modifyMiddleware(InsertAnnotation)).Methods("PUT")
	router.Handle(prefix+"/annotation/{id}", modifyMiddleware(DeleteAnnotation)).Methods("DELETE")
	router.Handle(prefix+"/annotation/values/{field}", readMiddleware(GetFieldValues)).Methods("GET")
	if !enableUI {
		return nil
	}
	webFS := FS(useLocalAssets)
	index, err := webFS.Open("/static/index.html")
	if err != nil {
		return fmt.Errorf("Error opening static file: %v", err)
	}
	indexHTML, err = ioutil.ReadAll(index)
	if err != nil {
		return err
	}
	router.PathPrefix("/static/").Handler(http.FileServer(webFS))
	router.PathPrefix("/").Handler(readMiddleware(Index)).Methods("GET")
	return nil
}

func Index(w http.ResponseWriter, r *http.Request) {
	w.Write(indexHTML)
}

//Web Section
var (
	indexHTML []byte
	backends  = []backend.Backend{}
)

func InsertAnnotation(w http.ResponseWriter, req *http.Request) {
	var a annotate.Annotation
	var ea annotate.EpochAnnotation
	epochFmt := false
	id := mux.Vars(req)["id"]

	// Need to read the request body twice to try both formats, so tee the
	b := bytes.NewBuffer(make([]byte, 0))
	tee := io.TeeReader(req.Body, b)
	d := json.NewDecoder(tee)
	errRegFmt := d.Decode(&a)
	if errRegFmt != nil {
		d := json.NewDecoder(b)
		errEpochFmt := d.Decode(&ea)
		if errEpochFmt != nil {
			serveError(w, fmt.Errorf("Could not unmarhsal json in RFC3339 fmt or Epoch fmt: %v, %v", errRegFmt, errEpochFmt))
			return
		}
		a = ea.AsAnnotation()
		epochFmt = true
	}
	if a.Id != "" && id != "" && a.Id != id {
		serveError(w, fmt.Errorf("conflicting ids in request: url id %v, body id %v", id, a.Id))
		return
	}
	if id != "" { // If we got the id in the url
		a.Id = id
	}
	if a.IsOneTimeSet() {
		a.MatchTimes()
	}
	if a.IsTimeNotSet() {
		a.SetNow()
	}
	err := a.ValidateTime()
	if err != nil {
		serveError(w, err)
		return
	}
	if a.Id == "" { //if Id isn't set, this is a new Annotation
		a.Id = uuid.NewV4().String()
	} else { // Make sure annotation exists if not new
		for _, b := range backends {
			//TODO handle multiple backends
			_, found, err := b.GetAnnotation(a.Id)
			if err == nil && !found {
				serveError(w, fmt.Errorf("could not find annotation with id %v to update: %v", a.Id, err))
				return
			}
			if err != nil {
				serveError(w, err)
				return
			}
		}
	}
	for _, b := range backends {
		log.Println("Inserting", a)
		err := b.InsertAnnotation(&a)
		//TODO Collect errors and insert into the backends that we can
		if err != nil {
			serveError(w, err)
			return
		}
	}
	if err = format(&a, w, epochFmt); err != nil {
		serveError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	return
}

func format(a *annotate.Annotation, w http.ResponseWriter, epochFmt bool) (e error) {
	if epochFmt {
		e = json.NewEncoder(w).Encode(a.AsEpochAnnotation())
	} else {
		e = json.NewEncoder(w).Encode(a)
	}
	return
}

func formatPlural(a annotate.Annotations, w http.ResponseWriter, epochFmt bool) (e error) {
	if epochFmt {
		e = json.NewEncoder(w).Encode(a.AsEpochAnnotations())
	} else {
		e = json.NewEncoder(w).Encode(a)
	}
	return
}

func GetAnnotation(w http.ResponseWriter, req *http.Request) {
	var a *annotate.Annotation
	var err error
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(req)["id"]
	for _, b := range backends {
		var found bool
		a, found, err = b.GetAnnotation(id)
		//TODO Collect errors and insert into the backends that we can
		if err == nil && !found {
			serve404(w)
			return
		}
		if err != nil {
			serveError(w, err)
			return
		}
	}
	err = format(a, w, req.URL.Query().Get("Epoch") == "1")
	if err != nil {
		serveError(w, err)
		return
	}
	return
}

func DeleteAnnotation(w http.ResponseWriter, req *http.Request) {
	id := mux.Vars(req)["id"]
	if id == "" {
		serveError(w, fmt.Errorf("id required"))
	}
	for _, b := range backends {
		err := b.DeleteAnnotation(id)
		//TODO Make sure it is deleted from at least one backend?
		if err != nil {
			serveError(w, err)
			return
		}
	}
}

func GetFieldValues(w http.ResponseWriter, req *http.Request) {
	values := []string{}
	var err error
	w.Header().Set("Content-Type", "application/json")
	field := mux.Vars(req)["field"]
	for _, b := range backends {
		values, err = b.GetFieldValues(field)
		//TODO Collect errors and insert into the backends that we can
		//TODO Unique Results from all backends
		if err != nil {
			serveError(w, err)
			return
		}
	}
	err = json.NewEncoder(w).Encode(values)
	if err != nil {
		serveError(w, err)
		return
	}
	return
}

func GetAnnotations(w http.ResponseWriter, req *http.Request) {
	var a annotate.Annotations
	var startT time.Time
	var endT time.Time
	var err error
	values := req.URL.Query()
	for param := range values {
		sp := strings.Split(param, ":")
		switch sp[0] {
		case annotate.StartDate:
		case annotate.EndDate:
		case annotate.Source:
		case annotate.Host:
		case annotate.CreationUser:
		case annotate.Owner:
		case annotate.Category:
		case annotate.Url:
		case annotate.Message:
		case "Epoch":
		default:
			serveError(w, fmt.Errorf("%v is not a valid query field", param))
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	// Time
	start := values.Get(annotate.StartDate)
	end := values.Get(annotate.EndDate)
	if start != "" {
		s, rfcErr := time.Parse(time.RFC3339, start)
		if rfcErr != nil {
			epoch, epochErr := strconv.ParseInt(start, 10, 64)
			if epochErr != nil {
				serveError(w, fmt.Errorf("couldn't parse StartDate as RFC3339 or epoch: %v, %v", rfcErr, epochErr))
				return
			}
			s = time.Unix(epoch, 0)
		}
		startT = s
	}
	if end != "" {
		s, rfcErr := time.Parse(time.RFC3339, end)
		if rfcErr != nil {
			epoch, epochErr := strconv.ParseInt(end, 10, 64)
			if epochErr != nil {
				serveError(w, fmt.Errorf("couldn't parse EndDate as RFC3339 or epoch: %v, %v", rfcErr, epochErr))
				return
			}
			s = time.Unix(epoch, 0)
		}
		endT = s
	}
	if end == "" {
		endT = time.Now().UTC()
	}
	if start == "" {
		startT = time.Now().Add(-time.Hour * 24)
	}

	// Queryable Fields
	filters := []backend.FieldFilter{}
	for param := range values {
		sp := strings.Split(param, ":")
		switch sp[0] {
		case annotate.Source, annotate.Host, annotate.CreationUser, annotate.Owner, annotate.Category, annotate.Message:
		default:
			continue
		}
		filter := backend.FieldFilter{Field: sp[0], Value: values.Get(param)}
		if len(sp) > 1 {
			filter.Verb = sp[1]
		}
		if len(sp) > 2 {
			filter.Not = true
		}
		filters = append(filters, filter)
	}

	// Execute
	for _, b := range backends {
		a, err = b.GetAnnotations(&startT, &endT, filters...)
		//TODO Collect errors and insert into the backends that we can
		if err != nil {
			serveError(w, err)
			return
		}
	}

	// Encode
	if err := formatPlural(a, w, values.Get("Epoch") == "1"); err != nil {
		serveError(w, err)
		return
	}
	return
}

func serveError(w http.ResponseWriter, err error) {
	jsonError := struct {
		Error string `json:"error"`
	}{
		err.Error(),
	}
	b, _ := json.Marshal(jsonError)
	http.Error(w, string(b), http.StatusInternalServerError)
}

func serve404(w http.ResponseWriter) {
	jsonError := struct {
		Error string `json:"error"`
	}{
		"not found",
	}
	b, _ := json.Marshal(jsonError)
	http.Error(w, string(b), http.StatusNotFound)
}
