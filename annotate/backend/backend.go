package backend

import (
	"fmt"
	"time"

	"bosun.org/annotate"
)

type Backend interface {
	InsertAnnotation(a *annotate.Annotation) error
	GetAnnotation(id string) (*annotate.Annotation, bool, error)
	GetAnnotations(start, end *time.Time, filters ...FieldFilter) (annotate.Annotations, error)
	DeleteAnnotation(id string) error
	GetFieldValues(field string) ([]string, error)
	InitBackend() error
}

const docType = "annotation"

var unInitErr = fmt.Errorf("backend has not been initialized")

type FieldFilter struct {
	Field string
	Verb  string
	Not   bool
	Value string
}

const Is = "Is"
const Empty = "Empty"
