package protocol_test

import (
	"testing"
	"time"

	. "github.com/xtls/xray-core/common/protocol"
)

func TestGenerateRandomInt64InRange(t *testing.T) {
	base := time.Now().Unix()
	delta := 100
	generator := NewTimestampGenerator(Timestamp(base), delta)

	for range 100 {
		val := int64(generator())
		if val > base+int64(delta) || val < base-int64(delta) {
			t.Error(val, " not between ", base-int64(delta), " and ", base+int64(delta))
		}
	}
}
