### Goweek

[![Build Status](https://travis-ci.org/grsmv/goweek.svg)](https://travis-ci.org/grsmv/goweek)
![Goreport](http://goreportcard.com/badge/grsmv/goweek)

Simple ISO 8601-compatible library for working with week entities for Go programming language. 
Rewrite of old [Ruby gem](https://github.com/grsmv/week)

#### Usage:

```go
// importing:
import "github.com/grsmv/goweek"

// initializing goweek.Week struct:
//                          year 
//                          |     week number (starting from 1)
//                          |     |
week, err := goweek.NewWeek(2015, 46)

// retrieving slice with days (`time.Time` instances) for a given week:
week.Days()

// retrieving `goweek.Week` instance for a next week:
nextWeek, err := week.Next()

// retrieving `goweek.Week` instance for a previous week:
previousWeek, err := week.Previous()
```

#### License:

```
Copyright (c) 2015 Serhii Herasymov

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```
