package httpext_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/ninedraft/httpext"
	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	t.Parallel()

	rw := &httptest.ResponseRecorder{}

	Error(rw, http.StatusTeapot)

	require.EqualValuesf(t, http.StatusTeapot, rw.Result().StatusCode, "result status code")
}
