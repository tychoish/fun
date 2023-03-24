package router

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

var counter = &atomic.Int64{}

const charSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numChars = len(charSet)
const idLenght = 32

func GenerateID() string {
	builder := &strings.Builder{}

	builder.WriteString(fmt.Sprint(time.Now().UnixMicro()))
	builder.WriteString("-")
	builder.WriteString(fmt.Sprint(counter.Add(1)))
	builder.WriteString("-")

	for builder.Len() < idLenght {
		builder.WriteByte(charSet[rand.Intn(numChars)])
	}
	return builder.String()
}
