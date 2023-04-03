package router

import (
	"math/rand"
	"time"
)

func populateID(id string) string {
	if id != "" {
		return id
	}
	return GenerateID()
}

func randomInterval(num int64) time.Duration {
	return time.Duration(num)*time.Millisecond + time.Duration(rand.Int63n(int64(num*int64(time.Millisecond))))
}
