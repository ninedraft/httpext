package httpext

import "net/http"

// Error replies to the request with the specified error message and HTTP code.
//
//	Error(w, http.StatusNotFound) = http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
func Error(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
