package logging

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"gopkg.in/errgo.v1"
)

var closeChan chan<- os.Signal

func SetCloseChan(c chan<- os.Signal) {
	closeChan = c
}

// CheckFatal exits the process if err != nil
func CheckFatal(err error) {
	if err != nil {
		pc, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "?"
			line = 0
		}
		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn == nil {
			fnName = "?()"
		} else {
			dotName := filepath.Ext(fn.Name())
			fnName = strings.TrimLeft(dotName, ".") + "()"
		}
		logrus.Errorf("%s:%d %s", file, line, fnName)
		logrus.Error("Fatal Error:", errgo.Details(err))
		if closeChan != nil {
			logrus.Warn("Sending close message")
			closeChan <- os.Interrupt
		}
		os.Exit(1)
	}
}

var Underlying = logrus.New()

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging(w io.Writer) {
	if w != nil {
		Underlying.Out = io.MultiWriter(os.Stderr, w)
	}
	if runtime.GOOS == "windows" { // colored ttys are rare on windows...
		Underlying.Formatter = &logrus.TextFormatter{DisableColors: true}
	}
	if lvl := os.Getenv("CRYPTIX_LOGLVL"); lvl != "" {
		l, err := logrus.ParseLevel(lvl)
		if err != nil {
			logrus.Errorf("logging: could not parse lvl from env, defaulting to debug: %s", err)
			l = logrus.DebugLevel
		}
		Underlying.Level = l
	}
}

// Logger returns an Entry where the module field is set to name
func Logger(name string) *logrus.Entry {
	if name == "" {
		Underlying.Warn("missing name parameter")
		name = "undefined"
	}
	return Underlying.WithField("module", name)
}
