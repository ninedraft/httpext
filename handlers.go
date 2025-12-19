package httpext

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// Error replies to the request with the specified error message and HTTP code.
//
//	Error(w, http.StatusNotFound) = http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
func Error(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func ServeBytes(filename string, data []byte) http.HandlerFunc {
	modTime := time.Now().UTC()

	pool := &sync.Pool{}

	getReader := func() *bytes.Reader {
		reader, _ := pool.Get().(*bytes.Reader)
		if reader == nil {
			reader = &bytes.Reader{}
		}
		reader.Reset(data)

		return reader
	}

	putReader := func(reader *bytes.Reader) {
		pool.Put(reader)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		re := getReader()
		defer putReader(re)

		http.ServeContent(w, r, filename, modTime, re)
	}
}
