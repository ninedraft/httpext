package httpext_test

import (
	"net/http/httptest"
	"testing"

	. "github.com/ninedraft/httpext"
	"github.com/stretchr/testify/require"
)

func TestBindValidate_BindsAllSources(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?q=answer&count=7", nil)
	req.Header.Set("X-Token", "secret")
	req.SetPathValue("id", "42")
	req.Pattern = "/items/{id}"

	target := &bindValidator{}
	err := BindValidate(req, target)

	require.NoError(t, err)
	require.Equal(t, "answer", target.Query)
	require.Equal(t, 7, target.Count)
	require.Equal(t, "secret", target.Token)
	require.Equal(t, 42, target.ID)
	require.True(t, target.validated, "validate hook is executed")
}

func TestBindValidate_ReturnsQueryScanError(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items?count=bad", nil)
	target := &bindValidator{}

	err := BindValidate(req, target)

	require.Error(t, err)
	require.Contains(t, err.Error(), "scanning query \"count\"")
	require.False(t, target.validated, "validate is skipped when binding fails")
}

func TestBindValidate_ReturnsHeaderScanError(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	req.Header.Set("X-Limit", "boom")

	target := &headerValidator{}
	err := BindValidate(req, target)

	require.Error(t, err)
	require.Contains(t, err.Error(), "scanning header \"X-Limit\"")
	require.False(t, target.validated, "validate is skipped when binding fails")
}

func TestBindValidate_BindsPathValuesWithoutHeaderTags(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items/13", nil)
	req.SetPathValue("id", "13")
	req.Pattern = "/items/{id}"

	target := &pathValidator{}
	err := BindValidate(req, target)

	require.NoError(t, err)
	require.Equal(t, 13, target.ID)
	require.True(t, target.validated)
}

func TestBindValidate_PathRequiresPatternPlaceholder(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	req.SetPathValue("id", "44")
	req.Pattern = "/items"

	target := &pathValidator{}
	err := BindValidate(req, target)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrMissingPatternValue)
	require.False(t, target.validated)
}

func TestBindValidate_PathValueScanError(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	req.SetPathValue("id", "abc")
	req.Pattern = "/items/{id}"

	target := &pathValidator{}
	err := BindValidate(req, target)

	require.Error(t, err)
	require.Contains(t, err.Error(), "scanning path value \"id\"")
	require.False(t, target.validated)
}

func TestBindValidate_PanicsOnNilTarget(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)

	require.Panics(t, func() {
		BindValidate(req, (*bindValidator)(nil))
	})
}

func TestBindValidate_PanicsOnNonPointerTarget(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/items", nil)
	target := valueValidator{}

	require.Panics(t, func() {
		BindValidate(req, target)
	})
}

type bindValidator struct {
	Query string `query:"q"`
	Count int    `query:"count"`
	Token string `header:"X-Token"`
	ID    int    `path:"id"`

	validated bool
}

func (t *bindValidator) Validate() error {
	t.validated = true
	return nil
}

type headerValidator struct {
	Limit int `header:"X-Limit"`

	validated bool
}

func (t *headerValidator) Validate() error {
	t.validated = true
	return nil
}

type pathValidator struct {
	ID int `path:"id"`

	validated bool
}

func (t *pathValidator) Validate() error {
	t.validated = true
	return nil
}

type valueValidator struct {
	Field string `query:"field"`
}

func (valueValidator) Validate() error { return nil }
