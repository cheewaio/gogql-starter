package response

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
)

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

func Success[T any](_ context.Context, in *T) (*T, error) {
	setFields(in, true, 200, "mutation succeeded")
	_, file, line, _ := runtime.Caller(1)
	slog.Debug("mutation succeeded", "caller", fmt.Sprintf("%s:%d", file, line))
	return in, nil
}

func Failure[T any](_ context.Context, in *T, err error) (*T, error) {
	setFields(in, false, 400, "mutation failed")
	_, file, line, _ := runtime.Caller(1)
	slog.Error("mutation failed", "error", err, "caller", fmt.Sprintf("%s:%d", file, line))
	return in, nil
}
