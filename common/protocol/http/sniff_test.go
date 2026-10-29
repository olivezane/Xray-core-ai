package http_test

import (
	"context"
	"testing"

	. "github.com/xtls/xray-core/common/protocol/http"
)

func TestHTTPHeaders(t *testing.T) {
	cases := []struct {
		input  string
		domain string
		err    bool
	}{
		{
			input: `GET /tutorials/other/top-20-mysql-best-practices/ HTTP/1.1
Host: net.tutsplus.com
User-Agent: Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.1.5) Gecko/20091102 Firefox/3.5.5 (.NET CLR 3.5.30729)
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-us,en;q=0.5
Accept-Encoding: gzip,deflate
Accept-Charset: ISO-8859-1,utf-8;q=0.7,*;q=0.7
Keep-Alive: 300
Connection: keep-alive
Cookie: PHPSESSID=r2t5uvjq435r4q7ib3vtdjq120
Pragma: no-cache
Cache-Control: no-cache`,
			domain: "net.tutsplus.com",
		},
		{
			input: `POST /foo.php HTTP/1.1
Host: localhost
User-Agent: Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.1.5) Gecko/20091102 Firefox/3.5.5 (.NET CLR 3.5.30729)
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-us,en;q=0.5
Accept-Encoding: gzip,deflate
Accept-Charset: ISO-8859-1,utf-8;q=0.7,*;q=0.7
Keep-Alive: 300
Connection: keep-alive
Referer: http://localhost/test.php
Content-Type: application/x-www-form-urlencoded
Content-Length: 43
 
first_name=John&last_name=Doe&action=Submit`,
			domain: "localhost",
		},
		{
			input: `X /foo.php HTTP/1.1
Host: localhost
User-Agent: Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.1.5) Gecko/20091102 Firefox/3.5.5 (.NET CLR 3.5.30729)
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-us,en;q=0.5
Accept-Encoding: gzip,deflate
Accept-Charset: ISO-8859-1,utf-8;q=0.7,*;q=0.7
Keep-Alive: 300
Connection: keep-alive
Referer: http://localhost/test.php
Content-Type: application/x-www-form-urlencoded
Content-Length: 43
 
first_name=John&last_name=Doe&action=Submit`,
			domain: "",
			err:    true,
		},
		{
			input: `GET /foo.php HTTP/1.1
User-Agent: Mozilla/5.0 (Windows; U; Windows NT 6.1; en-US; rv:1.9.1.5) Gecko/20091102 Firefox/3.5.5 (.NET CLR 3.5.30729)
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-us,en;q=0.5
Accept-Encoding: gzip,deflate
Accept-Charset: ISO-8859-1,utf-8;q=0.7,*;q=0.7
Keep-Alive: 300
Connection: keep-alive
Referer: http://localhost/test.php
Content-Type: application/x-www-form-urlencoded
Content-Length: 43

Host: localhost
first_name=John&last_name=Doe&action=Submit`,
			domain: "",
			err:    true,
		},
		{
			input:  `GET /tutorials/other/top-20-mysql-best-practices/ HTTP/1.1`,
			domain: "",
			err:    true,
		},
		{
			// The header section ends at the first empty line, so a Host in the body is not a header (RFC 9112 Section 6).
			input:  "POST /x HTTP/1.1\r\nHost: good.com\r\nContent-Length: 15\r\n\r\nhost: evil.com\r\n",
			domain: "good.com",
		},
		{
			// PATCH is defined by RFC 5789, TRACE by RFC 9110 Section 9.3.8.
			input:  "PATCH /x HTTP/1.1\r\nHost: patch.com\r\n\r\n",
			domain: "patch.com",
		},
		{
			input:  "TRACE /x HTTP/1.1\r\nHost: trace.com\r\n\r\n",
			domain: "trace.com",
		},
		{
			// RFC 9112 §3.2.2: an absolute-form target carries the authority and it
			// overrides the Host field.
			input:  "GET http://target.com/x HTTP/1.1\r\nHost: other.com\r\n\r\n",
			domain: "target.com",
		},
		{
			// RFC 9112 §3.2.3: CONNECT uses the authority-form, so Host is optional.
			input:  "CONNECT connect.com:443 HTTP/1.1\r\n\r\n",
			domain: "connect.com",
		},
	}

	for _, test := range cases {
		header, err := SniffHTTP([]byte(test.input), context.TODO())
		if test.err {
			if err == nil {
				t.Errorf("Expect error but nil, in test: %v", test)
			}
		} else {
			if err != nil {
				t.Errorf("Expect no error but actually %s in test %v", err.Error(), test)
			}
			if header.Domain() != test.domain {
				t.Error("expected domain ", test.domain, " but got ", header.Domain())
			}
		}
	}
}
