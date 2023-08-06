//go:build !race

package hdrhist_test

func without(int)       {}
func raceDetector() int { return 0 }
