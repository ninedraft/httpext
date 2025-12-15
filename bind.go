package httpext

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type validator interface {
	Validate() error
}

func BindValidate(req *http.Request, dst validator) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer {
		panic("[BindValidate] a non-nil struct pointer is expected as target")
	}
	if v.IsNil() {
		panic("[BindValidate] a non-nil struct pointer is expected as target")
	}
	v = v.Elem()

	if err := bind(req, v); err != nil {
		return err
	}

	return dst.Validate()
}

func bind(req *http.Request, dst reflect.Value) error {
	query := req.URL.Query()
	header := req.Header

	var errs []error

	dstT := dst.Type()
	for i := range dst.NumField() {
		field := dst.Field(i)
		fieldT := dstT.Field(i)

		if !fieldT.IsExported() || !field.CanAddr() {
			continue
		}

		fieldDst := field.Addr().Interface()

		tagQuery, ok := fieldT.Tag.Lookup("query")
		if ok {
			key := strings.TrimSpace(tagQuery)
			errs = append(errs, bindQuery(query, fieldDst, key))
		}

		tagHeader, ok := fieldT.Tag.Lookup("header")
		if ok {
			key := strings.TrimSpace(tagHeader)
			errs = append(errs, bindHeader(header, fieldDst, key))
		}

		if tagPathValue, ok := fieldT.Tag.Lookup("path"); ok {
			key := strings.TrimSpace(tagPathValue)
			errs = append(errs, bindPathValue(req, fieldDst, key))
		}
	}

	return errors.Join(errs...)
}

func bindQuery(query url.Values, dst any, key string) error {
	if !query.Has(key) {
		return nil
	}

	_, err := fmt.Sscan(query.Get(key), dst)
	if err != nil {
		return fmt.Errorf("scanning query %q: %w", key, err)
	}

	return err
}

func bindHeader(header http.Header, dst any, key string) error {
	values := header.Values(key)
	if len(values) == 0 {
		return nil
	}

	_, err := fmt.Sscan(values[0], dst)
	if err != nil {
		return fmt.Errorf("scanning header %q: %w", key, err)
	}

	return err
}

var ErrMissingPatternValue = errors.New("pattern doesn't contain key")

func bindPathValue(req *http.Request, dst any, key string) error {
	if !strings.Contains(req.Pattern, "{"+key+"}") &&
		!strings.Contains(req.Pattern, "{"+key+"...}") {
		return fmt.Errorf("%w %q", ErrMissingPatternValue, key)
	}

	_, err := fmt.Sscan(req.PathValue(key), dst)
	if err != nil {
		return fmt.Errorf("scanning path value %q: %w", key, err)
	}

	return err
}
