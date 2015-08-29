package debug

import (
	"io"
	"log"

	"github.com/cryptix/git-remote-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

/*
Copy of testing/iotest Read- and WriteLogger, but using %q instead of %x for printing
*/

type writeLogger struct {
	prefix string
	w      io.Writer
}

func (l *writeLogger) Write(p []byte) (n int, err error) {
	n, err = l.w.Write(p)
	if err != nil {
		log.Printf("%s %q: %v", l.prefix, string(p[0:n]), err)
	} else {
		log.Printf("%s %q", l.prefix, string(p[0:n]))
	}
	return
}

// NewWriteLogger returns a writer that behaves like w except
// that it logs (using log.Printf) each write to standard error,
// printing the prefix and the hexadecimal data written.
func NewWriteLogger(prefix string, w io.Writer) io.Writer {
	return &writeLogger{prefix, w}
}

type readLogger struct {
	prefix string
	r      io.Reader
}

func (l *readLogger) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	if err != nil {
		log.Printf("%s %q: %v", l.prefix, string(p[0:n]), err)
	} else {
		log.Printf("%s %q", l.prefix, string(p[0:n]))
	}
	return
}

// NewReadLogger returns a reader that behaves like r except
// that it logs (using log.Print) each read to standard error,
// printing the prefix and the hexadecimal data written.
func NewReadLogger(prefix string, r io.Reader) io.Reader {
	return &readLogger{prefix, r}
}

// logrus version
// ==============

type readLogrus struct {
	e *logrus.Entry
	r io.Reader
}

func (l *readLogrus) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	if err != nil {
		l.e.WithField("read", string(p[0:n])).WithField("err", err).Debug("errored logRead")
	} else {
		l.e.WithField("read", string(p[0:n])).Debug("logRead")
	}
	return
}

func NewReadLogrus(e *logrus.Entry, r io.Reader) io.Reader {
	return &readLogrus{e, r}
}

type writeLogrus struct {
	e *logrus.Entry
	w io.Writer
}

func (l *writeLogrus) Write(p []byte) (n int, err error) {
	n, err = l.w.Write(p)
	if err != nil {
		l.e.WithField("write", string(p[0:n])).WithField("err", err).Debug("errored logWrite")
	} else {
		l.e.WithField("write", string(p[0:n])).Debug("logWrite")
	}
	return
}

func NewWriteLogrus(e *logrus.Entry, w io.Writer) io.Writer {
	return &writeLogrus{e, w}
}
