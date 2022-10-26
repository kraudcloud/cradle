// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

type Formatter struct{}

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	var prefix string
	switch entry.Level {
	case logrus.PanicLevel:
		prefix = "<0>"
	case logrus.FatalLevel:
		prefix = "<1>"
	case logrus.ErrorLevel:
		prefix = "<2>"
	case logrus.WarnLevel:
		prefix = "<4>"
	case logrus.InfoLevel:
		prefix = "<5>"
	case logrus.DebugLevel:
		prefix = "<6>"
	case logrus.TraceLevel:
		prefix = "<7>"
	}


	m := strings.TrimFunc(entry.Message, func(c rune) bool {
		return !(('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') || c == ' ' || c == '\033')
	})

	m = fmt.Sprintf("%s%s", prefix, m)
	if !strings.HasSuffix(m, "\n") {
		m += "\n"
	}
	return []byte(m), nil
}

var log = &logrus.Logger{
	Level:     logrus.DebugLevel,
	Out:       os.Stderr,
	Formatter: &logrus.TextFormatter{},
}
