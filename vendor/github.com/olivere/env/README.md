# What is it?

[![Build Status](https://travis-ci.org/olivere/env.svg?branch=master)](https://travis-ci.org/olivere/env)

The `env` package is to simplify reading environment variables with e.g.
the `flag` package.

Example:

In our `main`, we often allow users to initialize our programs via flags.
When users leave out the flags, we want to fall back to environment variables.
The `env` package simplifies this scenario.

```go
import (
    "flag"
    "fmt"
)

func main() {
	var (
		// Parse addr from flag, use HTTP_ADDR and ADDR env vars as fallback
		addr = flag.String("addr", env.String("127.0.0.1:3000", "HTTP_ADDR", "ADDR"), "Bind to this address")
	)
	flag.Parse()

	fmt.Println(*addr)
	// Output: 127.0.0.1:3000
}
```

# License

MIT. See [LICENSE](https://github.com/olivere/env/blob/master/LICENSE) file.
