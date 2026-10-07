package postback

import (
	"net/netip"
	"net/url"
	"testing"
)

func TestRenderPreservesEncodedQueryValues(t *testing.T) {
	target, err := Render(
		"https://receiver.example/pb?cid={SOURCE_CLICK_ID}&payout={SUM}&sub={s1}&token=static",
		map[string]string{"source_click_id": "a&b=1 +/%", "sum": "0.00125", "s1": "zone one"},
	)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("cid") != "a&b=1 +/%" || u.Query().Get("payout") != "0.00125" || u.Query().Get("sub") != "zone one" || u.Query().Get("token") != "static" {
		t.Fatalf(
			"unexpected query: %v",
			u.Query(),
		)
	}
}

func TestInvalidPostbackURLs(t *testing.T) {
	for _, value := range []string{"file:///etc/passwd", "http://localhost/pb", "http://127.0.0.1/pb", "http://[::ffff:127.0.0.1]/pb", "http://169.254.169.254/pb", "https://user:password@receiver.example/pb", "https://{host}/pb", "https://receiver.example/{cid}", "https://receiver.example/pb?x={unknown}", "https://receiver.example/pb?x={cid", "https://receiver.example/pb?{cid}=1"} {
		if err := ValidateURL(value); err == nil {
			t.Fatalf(
				"accepted invalid URL %q",
				value,
			)
		}
	}
}

func TestPublicAddresses(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "0.1.2.3", "240.0.0.1", "192.168.1.1", "::1", "fc00::1", "fe80::1", "::ffff:192.168.1.1"} {
		if PublicAddress(netip.MustParseAddr(value)) {
			t.Fatalf(
				"accepted private address %s",
				value,
			)
		}
	}
	if !PublicAddress(netip.MustParseAddr("8.8.8.8")) || !PublicAddress(netip.MustParseAddr("2606:4700:4700::1111")) {
		t.Fatal("rejected public address")
	}
}
