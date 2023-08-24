package internal

import "reflect"

// IsPtr uses reflection to determine if an object is a pointer.
func IsPtr(in any) bool { return reflect.ValueOf(in).Kind() == reflect.Pointer }

// IsNil uses reflection to determine if an object is nil. ()
func IsNil(in any) bool {
	v := reflect.ValueOf(in)
	switch v.Kind() { //nolint:exhaustive
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
