package router

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var counter = &atomic.Int64{}

const charSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numChars = len(charSet)
const idLenght = 32

var hostID string

func init() {
	host, _ := os.Hostname()
	pid := os.Getpid()
	hasher := sha256.New()
	hasher.Sum([]byte(host))
	_ = binary.Write(hasher, binary.LittleEndian, int32(pid))
	hostsum := hasher.Sum([]byte{})
	hostID = fmt.Sprintf("%x", hostsum[:4])
}

func resetCounter() { counter.Store(0) }

// MessageID produces a (mostly) unique identifier for messages.  The
// ID is composed of a unix timestamp, an atomic counter (per-process
// lifetime), the prefix of a per-proccess hash, and a random ASCII
// character sequence of at least 16 characters (or more if the total
// sequence is less than 32 characters), with 3 - seperators. This ID
// has the property of being highly-likely to be unique, strictly
// orderable on one machine, and generally orderable across multiple
// machines, and visually parseable for human readers. The ParseID
// function decomposes these IDs programatically.
//
// The weakness of this format include, the format is not maximally
// compact and though unlikely collisions are possible.
type MessageID string

// GenerateID produces a default from the current time and the global counter.
func GenerateID() MessageID { return MakeID(time.Now(), counter.Add(1)) }

// MakeID constructs a message ID from the provided timestamp and
// counter.
func MakeID(ts time.Time, count int64) MessageID {
	builder := &strings.Builder{}

	builder.WriteString(fmt.Sprint(ts.Unix()))
	builder.WriteString("-")
	builder.WriteString(hostID)
	builder.WriteString("-")
	builder.WriteString(fmt.Sprint(count))
	builder.WriteString("-")

	for i := 0; i < 16 && builder.Len() < idLenght; i++ {
		builder.WriteByte(charSet[rand.Intn(numChars)])
	}
	return MessageID(builder.String())
}

// ParseID takes a message ID and decomposes it into parts.
func (id MessageID) Parse() (time.Time, string, int64, string, error) {
	parts := strings.Split(string(id), "-")
	if len(parts) != 4 {
		return time.Time{}, "", 0, "", errors.New("invalid id")
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", 0, "", err

	}
	count, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return time.Time{}, "", 0, "", err

	}

	return time.Unix(ts, 0), parts[1], count, parts[3], nil
}

func populateID(id string) string {
	if id != "" {
		return id
	}
	return string(GenerateID())
}

func randomInterval(num int64) time.Duration {
	return time.Duration(num)*time.Millisecond + time.Duration(rand.Int63n(int64(num*int64(time.Millisecond))))
}
