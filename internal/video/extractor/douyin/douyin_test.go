package douyin

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hezebin/media-dl/internal/httpx"
)

func TestAddWebSignature(t *testing.T) {
	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	client.HTTP().Jar.SetCookies(&url.URL{Scheme: "https", Host: "www.douyin.com", Path: "/"}, []*http.Cookie{
		{Name: "uifid", Value: "uifid-test"},
		{Name: "s_v_web_id", Value: "verify-test"},
	})

	got, headers := addWebSignature(client, "aid=6383&a_bogus=A%2FB")
	if headers["uifid"] != "uifid-test" {
		t.Fatalf("uifid header = %q", headers["uifid"])
	}
	for _, part := range []string{"&verifyFp=verify-test", "&fp=verify-test", "&uifid=uifid-test", "&timestamp=", "&x-secsdk-web-signature="} {
		if !strings.Contains(got, part) {
			t.Fatalf("signed query missing %q: %s", part, got)
		}
	}

	timestamp := strings.TrimPrefix(strings.Split(strings.Split(got, "&x-secsdk-web-signature=")[0], "&timestamp=")[1], "")
	stamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || stamp <= 0 {
		t.Fatalf("timestamp = %q", timestamp)
	}
	signature := got[strings.LastIndex(got, "=")+1:]
	want := fmt.Sprintf("%x", md5.Sum([]byte("uifid-test_"+timestamp+"_"+webSignatureSalt+"_"+strings.Split(got, "&x-secsdk-web-signature=")[0])))
	if signature != want || headers["x-secsdk-web-signature"] != want {
		t.Fatalf("signature = %q, want %q", signature, want)
	}
}

func TestAddWebSignatureWithoutVisitorCookie(t *testing.T) {
	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	query := "aid=6383"
	got, headers := addWebSignature(client, query)
	if got != query || headers != nil {
		t.Fatalf("query/headers = %q/%v, want unchanged/nil", got, headers)
	}
}
