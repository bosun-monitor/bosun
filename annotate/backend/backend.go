package backend

import (
	"fmt"
	"time"

	"bosun.org/annotate"
)

// Backend is an interface for a store of annotations, such as Elasticsearch
type Backend interface {
	// InsertAnnotation inserts an annotation
	InsertAnnotation(a *annotate.Annotation) error
	// GetAnnotation gets the annotation with the given ID
	GetAnnotation(id string) (*annotate.Annotation, bool, error)
	// GetAnnotations gets annotations filtered by the arguments
	GetAnnotations(start, end *time.Time, filters ...FieldFilter) (annotate.Annotations, error)
	// DeleteAnnotation deletes the annotation with the given ID
	DeleteAnnotation(id string) error
	// GetFieldValues gets the values for a given field
	GetFieldValues(field string) ([]string, error)
	// InitBackend initalises the backend
	InitBackend() error
}

const docType = "annotation"

var unInitErr = fmt.Errorf("backend has not been initialized")

// FieldFilter is a filter on fields
type FieldFilter struct {
	Field string
	Verb  string
	Not   bool
	Value string
}

// Constants currently only used by Elasticsearch.
const (
	// FIXME: Should be moved to a more sensible place
	Is    = "Is"
	Empty = "Empty"
)
