package logging

import (
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/negroni"
)

type HTTPLogger struct {
	*logrus.Entry
}

func NewNegroni(l *logrus.Entry) *HTTPLogger {
	return &HTTPLogger{l}
}

func NewNegroniWithName(l *logrus.Entry, name string) *HTTPLogger {
	return &HTTPLogger{l.WithField("module", name)}
}

func (l *HTTPLogger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	start := time.Now()

	l.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Debug("Request started")

	next(rw, r)

	res := rw.(negroni.ResponseWriter)
	l.WithFields(logrus.Fields{
		"status": res.Status(),
		"took":   time.Since(start),
	}).Debug("Request completed")
}
