package risky

// Send executes the provided future function and sends its result to the channel.
func Send[T any](ch chan<- T, fut func() T) { ch <- fut() }

// Wait blocks until the channel produces a value, always discarding the value.
func Wait[T any](ch <-chan T) { <-ch }

// Recv receives and returns a value from the channel, blocking according to the Go channel semantics.
func Recv[T any](ch <-chan T) T { return <-ch }
