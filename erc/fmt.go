package erc

import (
	"fmt"
	"strings"

	"github.com/tychoish/fun/ers"
)

const (
	fmtWrapSuffix       = ": %w"
	fmtWrapSuffixLenght = len(fmtWrapSuffix)
)

// Errorf constructs an error with fmt.Errorf-like semantics, but that
// takes advantage of erc and ers error semantics.
//
// If there are no args, Errorf returns an ers.Error constant. If there
// are args but none implement the error interface, Errorf returns
// ers.Error(fmt.Sprintf(...)).  If any args are errors, Errorf returns
// an 'erc.Collector' where each error arg is added individually
// (wrapped as "[index]: <error>"), and the format string is rendered
// with placeholders substituted indicating the index of the erorr.
//
// As a special case, if there is exactly one error arg, it is the last
// arg, and the template ends with ": %w", the placeholder and colon are
// omitted and the error is added directly.
func Errorf(tmpl string, args ...any) error {
	if len(args) == 0 {
		return ers.Error(tmpl)
	}

	// Find arg positions that implement error
	var errPositions []int
	for i, a := range args {
		if _, ok := a.(error); ok {
			errPositions = append(errPositions, i)
		}
	}

	if len(errPositions) == 0 {
		return ers.Error(fmt.Sprintf(tmpl, args...))
	}

	var ec Collector

	// Special case: single error, last arg, template ends with ": %w"
	if len(errPositions) == 1 && errPositions[0] == len(args)-1 && strings.HasSuffix(tmpl, fmtWrapSuffix) {
		prefix := tmpl[:len(tmpl)-fmtWrapSuffixLenght]
		var msg string
		if len(args) > 1 {
			msg = fmt.Sprintf(strings.ReplaceAll(prefix, "%w", "%v"), args[:len(args)-1]...)
		} else {
			msg = prefix
		}
		if msg != "" {
			ec.New(msg)
		}
		ec.Push(args[errPositions[0]].(error))
		return ec.Err()
	}

	// General case: substitute error args with [idx] placeholders
	newArgs := make([]any, len(args))
	copy(newArgs, args)
	for i, pos := range errPositions {
		newArgs[pos] = fmt.Sprintf("[%d]", i)
	}
	msg := fmt.Sprintf(strings.ReplaceAll(tmpl, "%w", "%v"), newArgs...)
	if msg != "" {
		ec.New(msg)
	}
	for i, pos := range errPositions {
		ec.Wrapf(args[pos].(error), "[%d]", i)
	}

	return ec.Err()
}
