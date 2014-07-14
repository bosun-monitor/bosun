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
	"strings"
)

type Logger interface {
	Error(v string)
	Info(v string)
	Warning(v string)
	Fatal(v string)
}

type stdLog struct {
	log *log.Logger
}

func (s *stdLog) Fatal(v string) {
	s.log.Fatalln("fatal:", rmNl(v))
}

func (s *stdLog) Error(v string) {
	s.log.Println("error:", rmNl(v))
}

func (s *stdLog) Info(v string) {
	s.log.Println("info:", rmNl(v))
}

func (s *stdLog) Warning(v string) {
	s.log.Println("warning:", rmNl(v))
}

func rmNl(v string) string {
	if strings.HasSuffix(v, "\n") {
		v = v[:len(v)-1]
	}
	return v
}

var logging Logger = &stdLog{log: log.New(os.Stderr, "", log.LstdFlags)}

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

func output(f func(string), v ...interface{}) {
	f(fmt.Sprint(v...))
}

func outputf(f func(string), format string, v ...interface{}) {
	output(f, fmt.Sprintf(format, v...))
}

func outputln(f func(string), v ...interface{}) {
	output(f, fmt.Sprintln(v...))
}
