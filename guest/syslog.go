package main

import (
	"github.com/sirupsen/logrus"
	"os"
	"fmt"
)

type Formatter struct {}
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
	return []byte(fmt.Sprintf("%s%s\n", prefix, entry.Message)), nil
}


var log = &logrus.Logger{
	Level:		logrus.DebugLevel,
	Out:		os.Stderr,
	Formatter:	&Formatter{},
}
