package reverse

import (
	"crypto/rand"
	"io"

	"github.com/xtls/xray-core/common/dice"
)

func (c *Control) FillInRandom() {
	randomLength := dice.Roll(64)
	randomLength++
	c.Random = make([]byte, randomLength)
	_, _ = io.ReadFull(rand.Reader, c.Random)
}
