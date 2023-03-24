package router

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrNoHandler is returned in the error object in response to
	// messages that have no registered handler.
	ErrNoHandler = errors.New("no handler defined")
	// ErrNoProtocol is returend by dispatcher methods that dispatch a
	// message, if the protocol is not defined in the message.
	ErrNoProtocol = errors.New("no protocol specified")
)

// Message is the format of all routed requests.
type Message struct {
	// Protocol reflects the version and schema definition that
	// describes the message's type. While middlweare can mutate
	// these values, changing the protocol during message
	// processing does not change which handler or middleware are
	// selected. Zero values are not valid for messages, and will
	// not be dispatched.
	Protocol Protocol
	// ID provides a unique message identification, and always set
	// by the dispatcher.
	ID string
	// Payload is an untyped field that holds the body of the
	// message. This can either be marshalled data in an
	// interchange format (json, protobuf, etc.) or a Go
	// object. Sequences of middleware mutate the content of the
	// message before it is handled.
	Payload any
}

// Response is the structure that the dispatcher returns to callers of
// synchronous messages.
type Response struct {
	// Protocol reflects the version and schema definition that
	// describes the response's type. Zero values are not
	// valid for responses. The protocol for Messages should match
	// the protocol of the Response.
	//
	// The dispatcher, however, only populate the protocol on the
	// response if the handler does not set it.
	Protocol Protocol
	// ID is a unique ID for the message, which matches the ID of
	// the message. The dispatcher will always set the ID on the
	// response after the handler returns.
	ID string
	// Payload is the core data of the response.
	Payload any
	Error   error
}

// Protocol describes the format of the message and response, and is
// used by the dispatcher to select middleware, handlers, and interceptors
// to dispatch messages and responses to.
type Protocol struct {
	// Schema is a string that the Dispatcher uses for dispatching the
	// message to the relevant middlware and handlers. Middleware
	// and Handlers are identified by the unique combination of
	// [Schema,Version], and the version makes it possible to
	// provide backwards-compatible protocols as schema's evolve.
	Schema string
	// Version reflects different iterations of a schema, to
	// proide more flexibility and compatibility as message
	// formats evolve.
	Version int
}

func (p Protocol) IsZero() bool   { return p.Schema == "" && p.Version == 0 }
func (p Protocol) String() string { return fmt.Sprintf("schema=%q, version=%d", p.Schema, p.Version) }

// Middleware functions handle (and modify) the message before it is
// passed to a handler functions. Middlewares can mutate the message
// before the handler recieves it, and is useful for access control,
// parsing the payload, and collecting metrics. Errors returned by a
// middleware function stop processing of a message, and are
// propagated back to the user.
type Middleware func(context.Context, *Message) error

// Interceptor process the result after the handler. Errors returned
// by interceptors abort further processing and populate the error
// field, of the response but do not otherwise modify the response.
type Interceptor func(context.Context, *Response) error

// Handlers process a message and return a response. Errors are
// propogated to the response. If no error is defined for a route, the
// response includes an ErrNoHandler error.
type Handler func(context.Context, Message) (Response, error)

// Error provides structured annotation of errors encountered during
// the processing of messages.
type Error struct {
	ID       string
	Protocol Protocol
	Err      error
}

func (e *Error) Unwrap() error { return e.Err }
func (e *Error) Error() string {
	return fmt.Sprintf("msg=%q, protocol=[%s]: %v", e.ID, e.Protocol, e.Err)
}
