package conf

import (
	"fmt"
	"testing"
)

func TestLex(t *testing.T) {
	input := `[testeoo] ntoh = eo02oen`
	l := lex("test", input)
	for i := range l.items {
		fmt.Println("item", i)
		if i.typ == itemEOF || i.typ == itemError {
			break
		}
	}
}
