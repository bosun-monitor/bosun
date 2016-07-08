# boolq
build simple bool expressions. Supports `!`, `AND`, `OR`, and `()` grouping. Individual items start with a letter and continue until whitespace. How you treat those items is up to you and is based on the Ask method.

# Example:

```
package main

import (
	"fmt"
	"log"

	"github.com/kylebrandt/boolq"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	f := foo{}
	ask := "(true AND true) AND !false"
	q, err := boolq.AskExpr(ask, f)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(q)
}

type foo struct{}

func (f foo) Ask(ask string) (bool, error) {
	switch ask {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("couldn't parse ask arg")
}
```