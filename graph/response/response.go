// Package response provides generic helpers for building consistent GraphQL
// mutation responses. It uses reflection to set common fields (Success, Code,
// Message) on any properly structured response type, avoiding boilerplate.
package response

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
)

// setFields uses reflection to populate the Success, Code, and Message fields
// on a response struct by name. This keeps mutation resolvers concise — they
// only need to return Success() or Failure() wrapping their response type.
func setFields[T any](in *T, success bool, code int, message string) {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if f := v.FieldByName("Success"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Bool {
		f.SetBool(success)
	}
	if f := v.FieldByName("Code"); f.IsValid() && f.CanSet() {
		switch f.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			f.SetInt(int64(code))
		}
	}
	if f := v.FieldByName("Message"); f.IsValid() && f.CanSet() {
		switch f.Kind() {
		case reflect.String:
			f.SetString(message)
		case reflect.Pointer:
			if f.Type().Elem().Kind() == reflect.String {
				msg := message
				f.Set(reflect.ValueOf(&msg))
			}
		}
	}
}

// Success wraps a mutation response with success=true, code=200, and records
// the caller location for debug logging.
func Success[T any](_ context.Context, in *T) (*T, error) {
	setFields(in, true, 200, "mutation succeeded")
	_, file, line, _ := runtime.Caller(1)
	slog.Debug("mutation succeeded", "caller", fmt.Sprintf("%s:%d", file, line))
	return in, nil
}

// Failure wraps a mutation response with success=false, code=400, and records
// the caller location and error for audit logging.
func Failure[T any](_ context.Context, in *T, err error) (*T, error) {
	setFields(in, false, 400, "mutation failed")
	_, file, line, _ := runtime.Caller(1)
	slog.Error("mutation failed", "error", err, "caller", fmt.Sprintf("%s:%d", file, line))
	return in, nil
}
