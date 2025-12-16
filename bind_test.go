package httpext

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinderQueryBindsBasicTypes(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?q=answer&count=7", nil)

	binder := newTestBinder(req)

	var (
		query string
		count int
	)

	binder.Query("q", &query).Query("count", &count)

	require.Equal(t, "answer", query)
	require.Equal(t, 7, count)
	require.NoError(t, binder.Err())
}

func TestBinderHeaderAndPathBinding(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items/42", nil)
	req.Header.Set("X-Token", "secret")
	req.SetPathValue("id", "42")

	binder := newTestBinder(req)

	var (
		token string
		id    int
	)

	binder.Header("X-Token", &token).Path("id", &id)

	require.Equal(t, "secret", token)
	require.Equal(t, 42, id)
	require.NoError(t, binder.Err())
}

func TestBinderQueryReportsMissingSource(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	binder := newTestBinder(req)

	var query string
	binder.Query("q", &query)

	err := binder.Err()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBindingMissing)
	require.Contains(t, err.Error(), `query "q"`)
}

func TestBinderQueryReportsScanError(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?count=bad", nil)
	binder := newTestBinder(req)

	var count int
	binder.Query("count", &count)

	err := binder.Err()
	require.Error(t, err)
	require.Contains(t, err.Error(), `query "count"`)
}

func TestBinderHeaderReportsMissingSource(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	binder := newTestBinder(req)

	var token string
	binder.Header("X-Token", &token)

	err := binder.Err()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBindingMissing)
	require.Contains(t, err.Error(), `header "X-Token"`)
}

func TestBinderPathReportsInvalidValue(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items/abc", nil)
	req.SetPathValue("id", "abc")
	binder := newTestBinder(req)

	var id int
	binder.Path("id", &id)

	err := binder.Err()
	require.Error(t, err)
	require.Contains(t, err.Error(), `path value "id"`)
}

func TestBinderSupportsTextUnmarshalers(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?value=payload", nil)
	binder := newTestBinder(req)

	var value upperText
	binder.Query("value", &value)

	require.Equal(t, upperText("PAYLOAD"), value)
	require.NoError(t, binder.Err())
}

func TestBinderErrJoinsMultipleErrors(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?count=bad", nil)
	req.Header.Set("X-Retry", "boom")

	binder := newTestBinder(req)

	var (
		missing string
		retry   int
	)

	binder.Query("missing", &missing).Header("X-Retry", &retry)

	err := binder.Err()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBindingMissing)
	require.Contains(t, err.Error(), `header "X-Retry"`)
}

func newTestBinder(req *http.Request) *Binder {
	return &Binder{
		req:   req,
		query: req.URL.Query(),
	}
}

type upperText string

func (u *upperText) UnmarshalText(text []byte) error {
	*u = upperText(strings.ToUpper(string(text)))
	return nil
}
