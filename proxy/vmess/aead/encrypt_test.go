package aead

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenVMessAEADHeader(t *testing.T) {
	TestHeader := []byte("Test Header")
	key := KDF16([]byte("Demo Key for Auth ID Test"), "Demo Path for Auth ID Test")
	var keyw [16]byte
	copy(keyw[:], key)
	sealed := SealVMessAEADHeader(keyw, TestHeader)

	AEADR := bytes.NewReader(sealed)

	var authid [16]byte

	_, _ = io.ReadFull(AEADR, authid[:])

	out, _, _, err := OpenVMessAEADHeader(keyw, authid, AEADR)

	fmt.Println(string(out))
	fmt.Println(err)
}

func TestOpenVMessAEADHeader2(t *testing.T) {
	TestHeader := []byte("Test Header")
	key := KDF16([]byte("Demo Key for Auth ID Test"), "Demo Path for Auth ID Test")
	var keyw [16]byte
	copy(keyw[:], key)
	sealed := SealVMessAEADHeader(keyw, TestHeader)

	AEADR := bytes.NewReader(sealed)

	var authid [16]byte

	_, _ = io.ReadFull(AEADR, authid[:])

	out, _, readen, err := OpenVMessAEADHeader(keyw, authid, AEADR)
	assert.Equal(t, len(sealed)-16-AEADR.Len(), readen)
	assert.Equal(t, string(TestHeader), string(out))
	assert.NoError(t, err)
}

func TestOpenVMessAEADHeader4(t *testing.T) {
	for i := range 61 {
		TestHeader := []byte("Test Header")
		key := KDF16([]byte("Demo Key for Auth ID Test"), "Demo Path for Auth ID Test")
		var keyw [16]byte
		copy(keyw[:], key)
		sealed := SealVMessAEADHeader(keyw, TestHeader)
		var sealedm [16]byte
		copy(sealedm[:], sealed)
		sealed[i] ^= 0xff
		AEADR := bytes.NewReader(sealed)

		var authid [16]byte

		_, _ = io.ReadFull(AEADR, authid[:])

		out, drain, readen, err := OpenVMessAEADHeader(keyw, authid, AEADR)
		assert.Equal(t, len(sealed)-16-AEADR.Len(), readen)
		assert.True(t, drain)
		require.Error(t, err)
		if err == nil {
			fmt.Println(">")
		}
		assert.Nil(t, out)
	}
}

func TestOpenVMessAEADHeader4Massive(t *testing.T) {
	for range 1000 {
		for i := range 61 {
			TestHeader := []byte("Test Header")
			key := KDF16([]byte("Demo Key for Auth ID Test"), "Demo Path for Auth ID Test")
			var keyw [16]byte
			copy(keyw[:], key)
			sealed := SealVMessAEADHeader(keyw, TestHeader)
			var sealedm [16]byte
			copy(sealedm[:], sealed)
			sealed[i] ^= 0xff
			AEADR := bytes.NewReader(sealed)

			var authid [16]byte

			_, _ = io.ReadFull(AEADR, authid[:])

			out, drain, readen, err := OpenVMessAEADHeader(keyw, authid, AEADR)
			assert.Equal(t, len(sealed)-16-AEADR.Len(), readen)
			assert.True(t, drain)
			require.Error(t, err)
			if err == nil {
				fmt.Println(">")
			}
			assert.Nil(t, out)
		}
	}
}
