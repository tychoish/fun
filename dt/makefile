.PHONY:all
all:tuple.go tuple_test.go

tuple%:pair%
	cp $< $@
	echo -e "// GENERATED FILE FROM PAIR IMPLEMENTATION\n$$(cat $@)" > $@
	sed -i 's/Pair/Tuple/g' $@
	sed -i 's/pair/tuple/g' $@
	sed -i 's/Key/One/g' $@
	sed -i -r 's/[^l.*).]Value/Two/g' $@
	sed -i 's/p.Value /p.Two /g' $@
	sed -i 's/Values/Twos/g' $@
	sed -i 's/comparable/any/g' $@
	goimports -w $@
	# go test ./... -v -count=1

.PHONY: reset
reset:
	rm -rf tuple.go tuple_test.go

