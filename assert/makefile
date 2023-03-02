check:
	mkdir -p check

check/%.go:%.go check
	cp $< $@
	[[ $@ = check/assert.go ]] && sed -i -e '1d' -e '2d' -e '3d' -e '4d' check/assert.go || true
	echo -e "// GENERATED FILE FROM ASSERTION PACKAGE\n$$(cat $@)" > $@
	sed -i 's/Fatal/Error/g' $@
	sed -i 's/package assert/package check/g' $@
	sed -i 's%"github.com/tychoish/fun/assert"%assert "github.com/tychoish/fun/assert/check"%' $@
	go test ./... -v -count=1

.PHONY: reset
reset:
	rm -rf check

.PHONY:all
all:check/assert.go check/assert_test.go
