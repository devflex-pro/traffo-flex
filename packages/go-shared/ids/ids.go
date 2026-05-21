package ids

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"
)

var fallbackSeq uint64

func New(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		seq := atomic.AddUint64(
			&fallbackSeq,
			1,
		)
		return prefix + "_" + strconv.FormatInt(
			time.Now().UnixNano(),
			36,
		) + strconv.FormatUint(
			seq,
			36,
		)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
