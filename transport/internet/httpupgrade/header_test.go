package httpupgrade

import (
	"net/http"
	"testing"
)

func TestHeaderHasToken(t *testing.T) {
	cases := []struct {
		values []string
		token  string
		want   bool
	}{
		{[]string{"Upgrade"}, "upgrade", true},
		// RFC 6455 §4.2.1: the token only has to be one item of the list.
		{[]string{"keep-alive, Upgrade"}, "upgrade", true},
		{[]string{"UPGRADE"}, "upgrade", true},
		{[]string{"keep-alive"}, "upgrade", false},
		{[]string{"upgraded"}, "upgrade", false},
		{nil, "upgrade", false},
	}

	for _, c := range cases {
		header := http.Header{}
		for _, v := range c.values {
			header.Add("Connection", v)
		}
		if got := headerHasToken(header, "Connection", c.token); got != c.want {
			t.Error("headerHasToken(", c.values, ", ", c.token, ") = ", got, " want ", c.want)
		}
	}
}
