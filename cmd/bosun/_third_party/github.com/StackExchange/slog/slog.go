// Package slog provides a cross-platform logging interface. It is designed to
// provide a universal logging interface on any operating system. It defaults to
// using the log package of the standard library, but can easily be used with
// other logging backends. Thus, we can use syslog on unicies and the event log
// on windows.
package slog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// LogLineNumber prints the file and line number of the caller.
	LogLineNumber = true
)

type Logger interface {
	Error(v string)
	Info(v string)
	Warning(v string)
	Fatal(v string)
}

type StdLog struct {
	Log *log.Logger
}

func (s *StdLog) Fatal(v string) {
	s.Log.Fatalln("fatal:", rmNl(v))
}

func (s *StdLog) Error(v string) {
	s.Log.Println("error:", rmNl(v))
}

func (s *StdLog) Info(v string) {
	s.Log.Println("info:", rmNl(v))
}

func (s *StdLog) Warning(v string) {
	s.Log.Println("warning:", rmNl(v))
}

func rmNl(v string) string {
	if strings.HasSuffix(v, "\n") {
		v = v[:len(v)-1]
	}
	return v
}

var logging Logger = &StdLog{Log: log.New(os.Stderr, "", log.LstdFlags)}

func Set(l Logger) {
	logging = l
}

func Info(v ...interface{}) {
	output(logging.Info, v...)
}

func Infof(format string, v ...interface{}) {
	outputf(logging.Info, format, v...)
}

func Infoln(v ...interface{}) {
	outputln(logging.Info, v...)
}

func Warning(v ...interface{}) {
	output(logging.Warning, v...)
}

func Warningf(format string, v ...interface{}) {
	outputf(logging.Warning, format, v...)
}

func Warningln(v ...interface{}) {
	outputln(logging.Warning, v...)
}

func Error(v ...interface{}) {
	output(logging.Error, v...)
}

func Errorf(format string, v ...interface{}) {
	outputf(logging.Error, format, v...)
}

func Errorln(v ...interface{}) {
	outputln(logging.Error, v...)
}

func Fatal(v ...interface{}) {
	output(logging.Fatal, v...)
	// Call os.Exit here just in case the logging package we are using doesn't.
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	outputf(logging.Fatal, format, v...)
	os.Exit(1)
}

func Fatalln(v ...interface{}) {
	outputln(logging.Fatal, v...)
	os.Exit(1)
}

func out(f func(string), s string) {
	if LogLineNumber {
		if _, filename, line, ok := runtime.Caller(3); ok {
			s = fmt.Sprintf("%s:%d: %v", filepath.Base(filename), line, s)
		}
	}
	f(s)
}

func output(f func(string), v ...interface{}) {
	out(f, fmt.Sprint(v...))
}

func outputf(f func(string), format string, v ...interface{}) {
	out(f, fmt.Sprintf(format, v...))
}

func outputln(f func(string), v ...interface{}) {
	out(f, fmt.Sprintln(v...))
}
