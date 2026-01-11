package dt

import "github.com/tychoish/fun/ers"

// ErrUninitializedContainer is the content of the panic produced when you
// attempt to perform an operation on an uninitialized sequence.
const ErrUninitializedContainer ers.Error = ers.Error("uninitialized container")

// ErrContainerStateImpossible is returned (often wrapped,) by implementations of container
// datatypes that have reached an impossible or invalid state.
const ErrContainerStateImpossible ers.Error = ers.Error("impossible container state")
