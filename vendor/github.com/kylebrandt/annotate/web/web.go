package web

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/kylebrandt/annotate"

	"github.com/gorilla/mux"
	"github.com/kylebrandt/annotate/backend"
	"github.com/twinj/uuid"
)

// esc -o static.go -pkg web static
func AddRoutes(router *mux.Router, prefix string, b []backend.Backend, enableUI, local bool) error {
	backends = b
	router.HandleFunc(prefix+"/annotation", InsertAnnotation).Methods("POST", "PUT")
	router.HandleFunc(prefix+"/annotation/query", GetAnnotations).Methods("GET")
	router.HandleFunc(prefix+"/annotation/{id}", GetAnnotation).Methods("GET")
	router.HandleFunc(prefix+"/annotation/{id}", InsertAnnotation).Methods("PUT")
	router.HandleFunc(prefix+"/annotation/{id}", DeleteAnnotation).Methods("DELETE")
	router.HandleFunc(prefix+"/annotation/values/{field}", GetFieldValues).Methods("GET")
	if !enableUI {
		return nil
	}
	webFS := FS(local)
	index, err := webFS.Open("/static/index.html")
	if err != nil {
		return fmt.Errorf("Error opening static file: %v", err)
	}
	indexHTML, err = ioutil.ReadAll(index)
	if err != nil {
		return err
	}
	router.PathPrefix("/static/").Handler(http.FileServer(webFS))
	router.PathPrefix("/").HandlerFunc(Index).Methods("GET")
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
	id := mux.Vars(req)["id"]
	d := json.NewDecoder(req.Body)
	err := d.Decode(&a)
	if err != nil {
		serveError(w, err)
		return
	}
	if a.Id != "" && id != "" && a.Id != id {
		serveError(w, fmt.Errorf("conflicting ids in request: url id %v, body id %v", id, a.Id))
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
	err = a.ValidateTime()
	if err != nil {
		serveError(w, err)
	}
	if a.Id == "" { //if Id isn't set, this is a new Annotation
		a.Id = uuid.NewV4().String()
	} else { // Make sure annotation exists if not new
		for _, b := range backends {
			//TODO handle multiple backends
			_, err := b.GetAnnotation(a.Id)
			if err != nil {
				serveError(w, fmt.Errorf("could not find annotation with id %v to update: %v", a.Id, err))
			}
		}
	}
	for _, b := range backends {
		err := b.InsertAnnotation(&a)
		//TODO Collect errors and insert into the backends that we can
		if err != nil {
			serveError(w, err)
		}
	}
	err = json.NewEncoder(w).Encode(a)
	if err != nil {
		serveError(w, err)
	}
	w.Header().Set("Content-Type", "application/json")
	return
}

func GetAnnotation(w http.ResponseWriter, req *http.Request) {
	var a *annotate.Annotation
	var err error
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(req)["id"]
	for _, b := range backends {
		a, err = b.GetAnnotation(id)
		//TODO Collect errors and insert into the backends that we can
		if err != nil {
			serveError(w, err)
		}
	}
	err = json.NewEncoder(w).Encode(a)
	if err != nil {
		serveError(w, err)
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
		}
	}
	err = json.NewEncoder(w).Encode(values)
	if err != nil {
		serveError(w, err)
	}
	return
}

func GetAnnotations(w http.ResponseWriter, req *http.Request) {
	var a annotate.Annotations
	var startT *time.Time
	var endT *time.Time
	var err error
	w.Header().Set("Content-Type", "application/json")
	// Time
	start := req.URL.Query().Get(annotate.StartDate)
	end := req.URL.Query().Get(annotate.EndDate)
	if start != "" {
		s, err := time.Parse(time.RFC3339, start)
		if err != nil {
			serveError(w, fmt.Errorf("error parsing StartDate %v: %v", start, err))
		}
		startT = &s
	}
	if end != "" {
		e, err := time.Parse(time.RFC3339, end)
		if err != nil {
			serveError(w, fmt.Errorf("error parsing EndDate %v: %v", end, err))
		}
		endT = &e
	}

	// Other Fields
	source := req.URL.Query().Get(annotate.Source)
	host := req.URL.Query().Get(annotate.Host)
	creationUser := req.URL.Query().Get(annotate.CreationUser)
	owner := req.URL.Query().Get(annotate.Owner)
	category := req.URL.Query().Get(annotate.Category)

	// Execute
	for _, b := range backends {
		a, err = b.GetAnnotations(startT, endT, source, host, creationUser, owner, category)
		//TODO Collect errors and insert into the backends that we can
		if err != nil {
			serveError(w, err)
		}
	}

	// Encode
	err = json.NewEncoder(w).Encode(a)
	if err != nil {
		serveError(w, err)
	}
	return
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
