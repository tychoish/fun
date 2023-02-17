package srv

import (
	"context"
	"testing"

	"github.com/tychoish/fun/erc"
)

func TestContext(t *testing.T) {
	t.Run("From", func(t *testing.T) {
		t.Run("Panics", func(t *testing.T) {
			ec := &erc.Collector{}
			func() {
				defer erc.Recover(ec)
				FromContext(context.Background())
			}()
			err := ec.Resolve()
			if err == nil {
				t.Error("should have panic'd")
			}
			if err.Error() != "panic: nil orchestrator" {
				t.Error("wrong panic value")
			}
		})
		t.Run("Fetch", func(t *testing.T) {
			orc := &Orchestrator{}

			ctx := context.Background()

			nctx := WithContext(ctx, orc)

			or := FromContext(nctx)
			if or == nil {
				t.Error("should not be nil")
			}
			if or != orc {
				t.Error("should be the same orchestrator")
			}
		})
	})
	t.Run("With", func(t *testing.T) {
		orc := &Orchestrator{}

		ctx := context.Background()

		nctx := WithContext(ctx, orc)
		if nctx == ctx {
			t.Error("should have replaced context")
		}

		nnctx := WithContext(nctx, orc)
		if nnctx != nctx {
			t.Error("should not replace context")
		}
		t.Run("Duplicate", func(t *testing.T) {
			ec := &erc.Collector{}
			func() {
				defer erc.Recover(ec)
				_ = WithContext(nctx, &Orchestrator{})
			}()
			err := ec.Resolve()
			if err == nil {
				t.Error("should have panic'd")
			}
			if err.Error() != "panic: multiple orchestrators" {
				t.Error("wrong panic value")
			}

		})
	})
}
