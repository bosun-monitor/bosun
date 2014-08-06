// Build sets scollector version information and should be run from the
// scollector directory.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	const path = "main.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					if len(spec.Names) != 1 || len(spec.Values) != 1 {
						continue
					}
					switch spec.Names[0].Name {
					case "VersionDate":
						switch value := spec.Values[0].(type) {
						case *ast.BasicLit:
							now := time.Now().UTC().Format("20060102150405")
							value.Value = now
						}
					case "VersionID":
						switch value := spec.Values[0].(type) {
						case *ast.BasicLit:
							rev, err := exec.Command("git", "rev-parse", "HEAD").Output()
							if err != nil {
								log.Fatal(err)
							}
							value.Value = fmt.Sprintf(`"%s"`, strings.TrimSpace(string(rev)))
						}
					}
				}
			}
		}
	}
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		log.Fatal(err)
	}
	fb, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(path, fb, info.Mode()); err != nil {
		log.Fatal(err)
	}
}
