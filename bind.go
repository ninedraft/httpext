package httpext

import (
	"encoding"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
)

var ErrBindingMissing = errors.New("missing binding source")

type Binder struct {
	req   *http.Request
	query url.Values
	errs  []error
}

func (binder *Binder) Err() error {
	return errors.Join(binder.errs...)
}

func (binder *Binder) Query(key string, dst any) *Binder {
	if !binder.query.Has(key) {
		err := fmt.Errorf("%w query %q", ErrBindingMissing, key)
		binder.errs = append(binder.errs, err)
	}

	if err := bindValues(binder.query[key], dst); err != nil {
		err := fmt.Errorf("%w: query %q", err, key)
		binder.errs = append(binder.errs, err)
	}

	return binder
}

func (binder *Binder) Header(key string, dst any) *Binder {
	if binder.req.Header.Get(key) == "" {
		err := fmt.Errorf("%w header %q", ErrBindingMissing, key)
		binder.errs = append(binder.errs, err)
	}

	if err := bindValues(binder.req.Header.Values(key), dst); err != nil {
		err := fmt.Errorf("%w: header %q", err, key)
		binder.errs = append(binder.errs, err)
	}

	return binder
}

func (binder *Binder) Path(key string, dst any) *Binder {
	values := []string{binder.req.PathValue(key)}

	if err := bindValues(values, dst); err != nil {
		err := fmt.Errorf("%w: path value %q", err, key)
		binder.errs = append(binder.errs, err)
	}

	return binder
}

var errNovalues = errors.New("no values")

func bindValues(values []string, dst any) error {
	if len(values) == 0 {
		return errNovalues
	}

	bindSingle := func(values []string, dst any) error {
		_, err := fmt.Sscan(values[0], dst)
		return err
	}

	if tum, ok := dst.(encoding.TextUnmarshaler); ok {
		bindSingle = func(values []string, dst any) error {
			return tum.UnmarshalText([]byte(values[0]))
		}
	}

	bind := bindSingle

	if isSlicePtr(dst) {
		bind = func(values []string, dst any) error {
			slice := reflect.ValueOf(dst).Elem()
			slice.SetLen(0)

			for i := range values {
				value := reflect.Zero(slice.Type().Elem())
				err := bindSingle(values[i:i+1], value.Addr())
				if err != nil {
					return fmt.Errorf("slice [%v]: %w", i, err)
				}

				slice = reflect.Append(slice, value)
			}

			return nil
		}
	}

	return bind(values, dst)
}

func isSlicePtr(v any) bool {
	vt := reflect.TypeOf(v)
	if vt.Kind() != reflect.Pointer {
		return false
	}

	vt = vt.Elem()
	if vt.Kind() != reflect.Slice {
		return false
	}

	if vt.Elem().Kind() == reflect.Int8 {
		// []byte, special case
		return false
	}

	return true
}
