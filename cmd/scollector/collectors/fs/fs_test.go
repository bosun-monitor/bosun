package fs

import (
	"strings"
	"testing"
)

var procFS = "nodev\tpstore\n" +
	"nodev\tmqueue\n" +
	"\text3\n" +
	"\text2\n" +
	"nodev\tbinfmt_misc\n" +
	"nodev\tautofs"

func TestGetPseudoFS(t *testing.T) {
	fs, err := getPseudoFS(strings.NewReader(procFS))
	if err != nil {
		t.Fatalf("should not return an error. Got: %v", err)
	}

	pseudo, ok := fs["pstore"]
	if ok == false {
		t.Fatalf("fs[\"pstore\"] should be present")
	}
	if pseudo == false {
		t.Fatalf("fs[\"pstore\"] should be true")
	}

	procFS = ""
	_, err = getPseudoFS(strings.NewReader(procFS))
	if err == nil {
		t.Fatalf("an empty file should return an error")
	}
}

func TestIsPseudo(t *testing.T) {
	fs, _ := getPseudoFS(strings.NewReader(procFS))

	for name, isPseudo := range fs {
		if isPseudo != fs.IsPseudo(name) {
			t.Fatalf("%v: got %v, should have %v", name, isPseudo, fs.IsPseudo(name))
		}
	}

	if fs.IsPseudo("doesn't exist") == false {
		t.Fatalf("Should consider nonexisting filesystems type as pseudofs")
	}
}

func TestParseLine(t *testing.T) {
	s, _ := parseFSLine("\ttiti\ttoto\ttata")
	if s != "" {
		t.Fatalf("should return an empty string: too many tabs")
	}
}
