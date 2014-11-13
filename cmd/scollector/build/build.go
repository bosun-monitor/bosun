// Build sets scollector version information and should be run from the
// scollector directory.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	const path = "main.go"
	mainfile, err := os.OpenFile(path, os.O_RDWR, 0660)
	if err != nil {
		log.Fatal(err)
	}
	defer mainfile.Close()

	var hash, id string
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, mainfile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	for _, d := range f.Decls {
		d, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		if d.Tok != token.CONST {
			continue
		}
		for _, spec := range d.Specs {
			spec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			if len(spec.Names) != 1 || len(spec.Values) != 1 {
				continue
			}

			value, ok := spec.Values[0].(*ast.BasicLit)
			if !ok {
				continue
			}

			switch spec.Names[0].Name {
			case "VersionDate":
				id = time.Now().UTC().Format("20060102150405")
				value.Value = id
			case "VersionID":
				rev, err := exec.Command("git", "rev-parse", "HEAD").Output()
				if err != nil {
					log.Fatal(err)
				}
				hash = fmt.Sprintf(`"%s"`, strings.TrimSpace(string(rev)))
				value.Value = hash
			}
		}
	}

	var config = printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	var buf bytes.Buffer
	if err := config.Fprint(&buf, fset, f); err != nil {
		log.Fatal(err)
	}

	if _, err := mainfile.Seek(0, os.SEEK_SET); err != nil {
		log.Fatal(err)
	}

	if _, err := io.Copy(mainfile, &buf); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("version:\n  hash: %s\n  id: %s\n", hash, id)
}
