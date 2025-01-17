package hdrhist_test

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/dt/hdrhist"
)

const num int64 = 10000000

func TestHighSigFig(t *testing.T) {
	defer without(raceDetector())

	input := []int64{
		459876, 669187, 711612, 816326, 931423, 1033197, 1131895, 2477317,
		3964974, 12718782,
	}

	hist := hdrhist.New(459876, 12718782, 5)
	for _, sample := range input {
		if err := hist.RecordValue(sample); err != nil {
			t.Error(err)
		}
	}

	if v, want := hist.ValueAtQuantile(50), int64(1048575); v != want {
		t.Errorf("Median was %v, but expected %v", v, want)
	}
}

func TestValueAtQuantile(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	data := []struct {
		q float64
		v int64
	}{
		{q: 50, v: 500223},
		{q: 75, v: 750079},
		{q: 90, v: 900095},
		{q: 95, v: 950271},
		{q: 99, v: 990207},
		{q: 99.9, v: 999423},
		{q: 99.99, v: 999935},
	}

	for _, d := range data {
		if v := h.ValueAtQuantile(d.q); v != d.v {
			t.Errorf("P%v was %v, but expected %v", d.q, v, d.v)
		}
	}

	if h.ValueAtQuantile(500) != h.ValueAtQuantile(100) {
		t.Error("cap quantiles at 100")
	}

	hist := hdrhist.New(4, 1024, 4)
	if n := hist.ValueAtQuantile(500); n != 0 {
		t.Error("cap quantiles at 100")
	}
}

func TestMean(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Mean(), 500000.013312; v != want {
		t.Errorf("Mean was %v, but expected %v", v, want)
	}
}

func TestStdDev(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.StdDev(), 288675.1403682715; v != want {
		t.Errorf("StdDev was %v, but expected %v", v, want)
	}
}

func TestTotalCount(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
		if v, want := h.TotalCount(), i+1; v != want {
			t.Errorf("TotalCount was %v, but expected %v", v, want)
		}
	}
}

func TestMax(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Max(), int64(1000447); v != want {
		t.Errorf("Max was %v, but expected %v", v, want)
	}
}

func TestReset(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	h.Reset()

	if v, want := h.Max(), int64(0); v != want {
		t.Errorf("Max was %v, but expected %v", v, want)
	}
}

func TestMerge(t *testing.T) {
	defer without(raceDetector())

	h1 := hdrhist.New(1, 1000, 3)
	h2 := hdrhist.New(1, 1000, 3)

	for i := 0; i < 100; i++ {
		if err := h1.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	for i := 100; i < 200; i++ {
		if err := h2.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	h1.Merge(h2)

	if v, want := h1.ValueAtQuantile(50), int64(99); v != want {
		t.Errorf("Median was %v, but expected %v", v, want)
	}
}

func TestMin(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Min(), int64(0); v != want {
		t.Errorf("Min was %v, but expected %v", v, want)
	}
}

func TestByteSize(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, 100000, 3)

	if v, want := h.ByteSize(), 65604; v != want {
		t.Errorf("ByteSize was %v, but expected %d", v, want)
	}
}

func TestRecordCorrectedValue(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, 100000, 3)

	if err := h.RecordCorrectedValue(10, 100); err != nil {
		t.Fatal(err)
	}

	if v, want := h.ValueAtQuantile(75), int64(10); v != want {
		t.Errorf("Corrected value was %v, but expected %v", v, want)
	}
}

func TestRecordCorrectedValueStall(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num/100, 3)

	if err := h.RecordCorrectedValue(1000, 100); err != nil {
		t.Fatal(err)
	}

	if v, want := h.ValueAtQuantile(75), int64(800); v != want {
		t.Errorf("Corrected value was %v, but expected %v", v, want)
	}
}

func TestCumulativeDistribution(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, num, 3)

	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	actual := h.CumulativeDistribution()
	expected := []hdrhist.Bracket{
		{Quantile: 0, Count: 1, ValueAt: 0},
		{Quantile: 50, Count: 500224, ValueAt: 500223},
		{Quantile: 75, Count: 750080, ValueAt: 750079},
		{Quantile: 87.5, Count: 875008, ValueAt: 875007},
		{Quantile: 93.75, Count: 937984, ValueAt: 937983},
		{Quantile: 96.875, Count: 969216, ValueAt: 969215},
		{Quantile: 98.4375, Count: 984576, ValueAt: 984575},
		{Quantile: 99.21875, Count: 992256, ValueAt: 992255},
		{Quantile: 99.609375, Count: 996352, ValueAt: 996351},
		{Quantile: 99.8046875, Count: 998400, ValueAt: 998399},
		{Quantile: 99.90234375, Count: 999424, ValueAt: 999423},
		{Quantile: 99.951171875, Count: 999936, ValueAt: 999935},
		{Quantile: 99.9755859375, Count: 999936, ValueAt: 999935},
		{Quantile: 99.98779296875, Count: 999936, ValueAt: 999935},
		{Quantile: 99.993896484375, Count: 1000000, ValueAt: 1000447},
		{Quantile: 100, Count: 1000000, ValueAt: 1000447},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("CF was %#v, but expected %#v", actual, expected)
	}
}

func TestDistribution(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(8, 1024, 3)

	for i := 0; i < 1024; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	actual := h.Distribution()
	if len(actual) != 128 {
		t.Errorf("Number of bars seen was %v, expected was 128", len(actual))
	}

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(actual[0], "=>", actual[len(actual)-1])
		}
	})

	for _, b := range actual {
		if b.Count != 8 {
			t.Errorf("Count per bar seen was %v, expected was 8", b.Count)
		}
	}
}

func TestNaN(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(1, 100000, 3)
	if math.IsNaN(h.Mean()) {
		t.Error("mean is NaN")
	}
	if math.IsNaN(h.StdDev()) {
		t.Error("stddev is NaN")
	}
}

func TestSignificantFigures(t *testing.T) {
	defer without(raceDetector())

	const sigFigs = 4
	h := hdrhist.New(1, 10, sigFigs)
	if h.SignificantFigures() != sigFigs {
		t.Errorf("Significant figures was %v, expected %d", h.SignificantFigures(), sigFigs)
	}
}

func TestLowestTrackableValue(t *testing.T) {
	defer without(raceDetector())

	const minVal = 2
	h := hdrhist.New(minVal, 10, 3)
	if h.LowestTrackableValue() != minVal {
		t.Errorf("LowestTrackableValue figures was %v, expected %d", h.LowestTrackableValue(), minVal)
	}
}

func TestHighestTrackableValue(t *testing.T) {
	defer without(raceDetector())

	const maxVal = 11

	h := hdrhist.New(1, maxVal, 3)
	if h.HighestTrackableValue() != maxVal {
		t.Errorf("HighestTrackableValue figures was %v, expected %d", h.HighestTrackableValue(), maxVal)
	}
}

func BenchmarkHistRecordValue(b *testing.B) {
	h := hdrhist.New(1, num, 3)
	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = h.RecordValue(100)
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		hdrhist.New(1, 120000, 3) // this could track 1ms-2min
	}
}

func TestUnitMagnitudeOverflow(t *testing.T) {
	defer without(raceDetector())

	h := hdrhist.New(0, 200, 4)
	if err := h.RecordValue(11); err != nil {
		t.Fatal(err)
	}
}

func TestSubBucketMaskOverflow(t *testing.T) {
	defer without(raceDetector())

	hist := hdrhist.New(2e7, 1e8, 5)
	for _, sample := range [...]int64{1e8, 2e7, 3e7} {
		if err := hist.RecordValue(sample); err != nil {
			t.Error(err)
		}
	}

	for q, want := range map[float64]int64{
		50:    33554431,
		83.33: 33554431,
		83.34: 100663295,
		99:    100663295,
	} {
		if got := hist.ValueAtQuantile(q); got != want {
			t.Errorf("got %d for %fth percentile. want: %d", got, q, want)
		}
	}
}

func TestExportImport(t *testing.T) {
	defer without(raceDetector())

	minVal := int64(1)
	maxVal := num
	sigfigs := 3
	h := hdrhist.New(minVal, maxVal, sigfigs)
	for i := int64(0); i < num/10; i++ {
		if err := h.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	s := h.Export()

	if v := s.LowestTrackableValue; v != minVal {
		t.Errorf("LowestTrackableValue was %v, but expected %v", v, minVal)
	}

	if v := s.HighestTrackableValue; v != maxVal {
		t.Errorf("HighestTrackableValue was %v, but expected %v", v, maxVal)
	}

	if v := int(s.SignificantFigures); v != sigfigs {
		t.Errorf("SignificantFigures was %v, but expected %v", v, sigfigs)
	}

	if imported := hdrhist.Import(s); !imported.Equals(h) {
		t.Error("Expected Hists to be equivalent")
	}

}

func TestEquals(t *testing.T) {
	defer without(raceDetector())

	h1 := hdrhist.New(1, num, 3)
	for i := int64(0); i < num/10; i++ {
		if err := h1.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	h2 := hdrhist.New(1, num, 3)
	for i := 0; i < 10000; i++ {
		if err := h1.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if h1.Equals(h2) {
		t.Error("Expected Histograms to not be equivalent")
	}

	h1.Reset()
	h2.Reset()

	if !h1.Equals(h2) {
		t.Error("Expected Histograms to be equivalent")
	}

}

func TestEqualsEdgeCase(t *testing.T) {
	defer without(raceDetector())

	first := hdrhist.New(1, num, 3)
	second := hdrhist.New(1, num, 3)
	for i := int64(0); i < 10000; i++ {
		if err := errors.Join(
			first.RecordValue(i),
			second.RecordValue(i),
		); err != nil {
			t.Fatal(err)
		}
	}

	if !first.Equals(second) {
		t.Fatal("should be equal")
	}

	if err := errors.Join(
		first.RecordValue(4),
		second.RecordValue(8),
	); err != nil {
		t.Fatal(err)
	}

	if first.Equals(second) {
		t.Fatal("should not be equal")
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("LargeValues", func(t *testing.T) {
		hist := hdrhist.New(0, 10, 2)
		if err := errors.Join(
			hist.RecordValues(-2000, 1),
			hist.RecordValues(2000, 1),
			hist.RecordValues(500, 2),
			hist.RecordCorrectedValue(2000, 3),
			hist.RecordCorrectedValue(-2000, 1),
			hist.RecordCorrectedValue(500, 2),
			hist.RecordCorrectedValue(2, 1),
			hist.RecordCorrectedValue(1, 1),
		); err == nil {
			t.Error("should have failed")
		}
	})
	t.Run("MergeDifferentSizes", func(t *testing.T) {
		histOne := hdrhist.New(0, 10, 2)
		if err := errors.Join(
			histOne.RecordValue(5),
			histOne.RecordValue(3),
			histOne.RecordValue(2),
		); err != nil {
			t.Error(err)
		}
		histTwo := hdrhist.New(100, 1000, 4)
		if err := errors.Join(
			histTwo.RecordValue(250),
			histTwo.RecordValue(500),
			histTwo.RecordValue(750),
		); err != nil {
			t.Error(err)
		}
		if n := histOne.Merge(histTwo); n == 0 {
			t.Error("no expected merge errors")
		}
	})
	t.Run("SignificantFiguresLimts", func(t *testing.T) {
		t.Run("Zero", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic")
				}
			}()
			hdrhist.New(0, 100, 0)
		})
		t.Run("Negative", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic")
				}
			}()
			hdrhist.New(0, 100, -10)
		})
		t.Run("Over", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic")
				}
			}()
			hdrhist.New(0, 100, 6)
		})
		t.Run("Larger", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic")
				}
			}()
			hdrhist.New(0, 10000000000000000, 20)
		})
		t.Run("Valid", func(t *testing.T) {
			// shouldn't panic

			assert.NotPanic(t, func() {

				hdrhist.New(0, 10000, 1)
				hdrhist.New(0, 10000, 2)
				hdrhist.New(0, 10000, 3)
				hdrhist.New(0, 10000, 4)
				hdrhist.New(0, 10000, 5)

			})
		})
	})
}

func TestRand(t *testing.T) {
	defer without(raceDetector())

	hist := hdrhist.New(0, 100, 3)
	for i := int64(0); i < 10000; i++ {
		if err := hist.RecordValue(rand.Int63n(101)); err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(0); i < 101; i++ {
		if err := hist.RecordValue(i); err != nil {
			t.Fatal(err)
		}
	}

	if hist.Min() != 0 {
		t.Error(hist.Min())
	}
	if hist.Max() != 100 {
		t.Error(hist.Max())
	}

	if hist.TotalCount() != 10000+101 {
		t.Error(hist.TotalCount())
	}
	exp := hist.Export()
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Log(exp)
	})

	if exp.HighestTrackableValue != 100 {
		t.Fail()
	}
	if exp.LowestTrackableValue != 0 {
		t.Fail()
	}
	if exp.SignificantFigures != 3 {
		t.Fail()
	}
}
