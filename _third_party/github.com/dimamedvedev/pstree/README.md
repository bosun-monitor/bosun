pstree
======

[![GoDoc](https://godoc.org/github.com/sbinet/pstree?status.svg)](https://godoc.org/github.com/sbinet/pstree)

`pstree` is a simple minded package to retrieve the process tree from a given
`PID`.

## Installation

```sh
sh> go get github.com/sbinet/pstree
```

## Documentation

Documentation is available on
[godoc](https://godoc.org):

 https://godoc.org/github.com/sbinet/pstree


## Example

```go
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/sbinet/pstree"
)

func main() {
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("could not retrieve pid: %v\n", err)
	}
	tree, err := pstree.New()
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	fmt.Printf("tree[%d]: %v\n", pid, tree.Procs[pid])
	display(pid, tree, 1)
}

func display(pid int, tree *pstree.Tree, indent int) {
	str := strings.Repeat("  ", indent)
	for _, cid := range tree.Procs[pid].Children {
		proc := tree.Procs[cid]
		fmt.Printf("%s%#v\n", str, proc)
		display(cid, tree, indent+1)
	}
}
```
