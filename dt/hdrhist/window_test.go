package hdrhist_test

import (
	"testing"

	"github.com/tychoish/fun/dt/hdrhist"
)

func TestWindowedHistogram(t *testing.T) {
	w := hdrhist.NewWindowed(2, 1, 1000, 3)

	for i := range 100 {
		if err := w.Current.RecordValue(int64(i)); err != nil {
			t.Error(err)
		}
	}
	w.Rotate()

	for i := 100; i < 200; i++ {
		if err := w.Current.RecordValue(int64(i)); err != nil {
			t.Error(err)
		}
	}
	w.Rotate()

	for i := 200; i < 300; i++ {
		if err := w.Current.RecordValue(int64(i)); err != nil {
			t.Error(err)
		}
	}

	if v, want := w.Merge().ValueAtQuantile(50), int64(199); v != want {
		t.Errorf("Median was %v, but expected %v", v, want)
	}
}

func BenchmarkWindowedHistogramRecordAndRotate(b *testing.B) {
	w := hdrhist.NewWindowed(3, 1, 10000000, 3)
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		if err := w.Current.RecordValue(100); err != nil {
			b.Fatal(err)
		}

		if i%100000 == 1 {
			w.Rotate()
		}
	}
}

func BenchmarkWindowedHistogramMerge(b *testing.B) {
	w := hdrhist.NewWindowed(3, 1, 10000000, 3)
	for i := range 10000000 {
		if err := w.Current.RecordValue(100); err != nil {
			b.Fatal(err)
		}

		if i%100000 == 1 {
			w.Rotate()
		}
	}
	b.ReportAllocs()

	for b.Loop() {
		w.Merge()
	}
}
