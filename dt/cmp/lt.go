// Package cmp provides comparators for sorting linked lists.
package cmp

import "time"

// OrderableNative describes all native types which (currently) support the
// < operator. To order custom types, use the OrderableUser interface.
//
// In the future an equivalent oderable type specification is likely
// to enter the standard library, which will supercede this type.
type OrderableNative interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~string
}

// Orderable allows users to define a method on their types which
// implement a method to provide a LessThan operation.
type Orderable[T any] interface{ LessThan(T) bool }

// LessThan describes a less than operation, typically provided by one
// of the following operations.
type LessThan[T any] func(a, b T) bool

// LessThanNative provides a wrapper around the < operator for types
// that support it, and can be used for sorting lists of compatible
// types.
func LessThanNative[T OrderableNative](a, b T) bool { return a < b }

// LessThanCustom converts types that implement OrderableUser
// interface.
func LessThanCustom[T Orderable[T]](a, b T) bool { return a.LessThan(b) }

// LessThanConverter provides a function to convert a non-orderable
// type to an orderable type. Use this for
func LessThanConverter[T any, S OrderableNative](converter func(T) S) LessThan[T] {
	return func(a, b T) bool { return LessThanNative(converter(a), converter(b)) }
}

// LessThanTime compares time using the time.Time.Before() method.
func LessThanTime(a, b time.Time) bool { return a.Before(b) }

// Reverse wraps an existing LessThan operator and reverses it's
// direction.
func Reverse[T any](fn LessThan[T]) LessThan[T] { return func(a, b T) bool { return !fn(a, b) } }
