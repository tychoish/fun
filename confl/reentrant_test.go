package confl

import (
	"flag"
	"os"
	"testing"

	"github.com/tychoish/fun/assert"
)

// resetCommandLine swaps flag.CommandLine for a fresh FlagSet and os.Args for
// argv, restoring both after the test. Registry.Register and Registry.ParseAll
// operate on flag.CommandLine, so exercising them for real (rather than
// through the fs-parameterized internals the rest of the suite uses) requires
// controlling it. Callers must not call t.Parallel(): these tests mutate
// process-global state and would race against each other.
func resetCommandLine(t *testing.T, argv ...string) {
	t.Helper()
	origFS := flag.CommandLine
	origArgs := os.Args
	flag.CommandLine = flag.NewFlagSet(argv[0], flag.ContinueOnError)
	os.Args = argv

	t.Cleanup(func() {
		flag.CommandLine = origFS
		os.Args = origArgs
	})
}

func TestRegistryParseAll_TwoIndependentStructs(t *testing.T) {
	type dbConfig struct {
		Host string `default:"localhost" flag:"db-host"`
	}
	type serverConfig struct {
		Port string `default:"8080" flag:"server-port"`
	}

	resetCommandLine(t, "app", "-db-host=db.internal", "-server-port=9090")

	var db dbConfig
	var srv serverConfig
	var reg Registry
	assert.NotError(t, reg.Register(&db))
	assert.NotError(t, reg.Register(&srv))
	assert.NotError(t, reg.ParseAll())

	assert.Equal(t, db.Host, "db.internal")
	assert.Equal(t, srv.Port, "9090")
}

// TestRegistryParseAll_OrderIndependent covers exactly the case that breaks
// two sequential Parse calls: a flag belonging to the struct registered
// SECOND appears BEFORE, on the command line, a flag belonging to the struct
// registered first. Two independent Parse calls would fail here, because the
// first call parses the whole command line before the second struct's flag
// has been registered.
func TestRegistryParseAll_OrderIndependent(t *testing.T) {
	type first struct {
		A string `default:"unset" flag:"a"`
	}
	type second struct {
		B string `default:"unset" flag:"b"`
	}

	resetCommandLine(t, "app", "-b=2", "-a=1")

	var f first
	var s second
	var reg Registry
	assert.NotError(t, reg.Register(&s)) // registered first; its flag appears second in argv
	assert.NotError(t, reg.Register(&f)) // registered second; its flag appears first in argv
	assert.NotError(t, reg.ParseAll())

	assert.Equal(t, f.A, "1")
	assert.Equal(t, s.B, "2")
}

func TestRegistryParseAll_RequiredCheckedForEveryStruct(t *testing.T) {
	type withRequired struct {
		Name string `flag:"name" required:"true"`
	}
	type other struct {
		X string `default:"y" flag:"x"`
	}

	resetCommandLine(t, "app")

	var r withRequired
	var o other
	var reg Registry
	assert.NotError(t, reg.Register(&r))
	assert.NotError(t, reg.Register(&o))
	assert.Error(t, reg.ParseAll())
}

func TestRegistryParseAll_DuplicateFlagNameErrors(t *testing.T) {
	type a struct {
		X string `flag:"x"`
	}
	type b struct {
		X string `flag:"x"`
	}

	resetCommandLine(t, "app")

	var ca a
	var cb b
	var reg Registry
	assert.NotError(t, reg.Register(&ca))
	assert.Error(t, reg.Register(&cb))
}

func TestRegistryParseAll_ZeroValueReady(t *testing.T) {
	type cfg struct {
		X string `default:"y" flag:"x"`
	}

	resetCommandLine(t, "app")

	var c cfg
	var reg Registry // zero value, no constructor needed
	assert.NotError(t, reg.Register(&c))
	assert.NotError(t, reg.ParseAll())
	assert.Equal(t, c.X, "y")
}

func TestParse_StillWorksForSingleStruct(t *testing.T) {
	type cfg struct {
		Name string `default:"world" flag:"name"`
	}

	resetCommandLine(t, "app", "-name=confl")

	var c cfg
	assert.NotError(t, Parse(&c))
	assert.Equal(t, c.Name, "confl")
}

func TestRegistryRegister_InvalidConfigErrors(t *testing.T) {
	resetCommandLine(t, "app")

	var reg Registry
	assert.Error(t, reg.Register(nil))
	assert.Error(t, reg.Register("not a pointer to a struct"))
}

func TestParse_InvalidConfigErrors(t *testing.T) {
	resetCommandLine(t, "app")

	assert.Error(t, Parse(nil))
}
