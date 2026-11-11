package main

import (
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"":                    false,
		"gzip":                true,
		"deflate, gzip;q=1.0": true,
		"br, zstd":            false,
		"gzip;q=0":            false,
		"gzip; q=0.000":       false,
		"GZIP;q=0.5":          true,
	}
	for hdr, want := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		if hdr != "" {
			r.Header.Set("Accept-Encoding", hdr)
		}
		if got := acceptsGzip(r); got != want {
			t.Errorf("%q: got %v, want %v", hdr, got, want)
		}
	}
}

func TestExitCompression(t *testing.T) {
	big := strings.Repeat("Nov 11 12:00:00 host unit[1]: some log line\n", 100)
	for _, tc := range []struct {
		name, text, enc string
		gz              bool
	}{
		{"small", "OK", "gzip", false},
		{"big no header", big, "", false},
		{"big gzip", big, "gzip", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tc.enc != "" {
				r.Header.Set("Accept-Encoding", tc.enc)
			}
			w := httptest.NewRecorder()
			checkResult{returncode: 0, text: tc.text}.exit(w, r, nil)
			res := w.Result()
			var body io.Reader = res.Body
			if got := res.Header.Get("Content-Encoding") == "gzip"; got != tc.gz {
				t.Fatalf("gzip=%v, want %v", got, tc.gz)
			}
			if tc.gz {
				zr, err := gzip.NewReader(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				body = zr
			}
			b, _ := io.ReadAll(body)
			if want := tc.text + "\nResult Code: 0\n"; string(b) != want {
				t.Fatalf("body mismatch: %d bytes", len(b))
			}
			if res.Header.Get("Vary") != "Accept-Encoding" {
				t.Fatal("missing Vary")
			}
		})
	}
}

