package douyin

import (
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestDouyinNetworkCookiesRequiresDomainBoundary(t *testing.T) {
	values := douyinNetworkCookies([]*network.Cookie{
		{Name: "kept", Value: "1", Domain: ".douyin.com"},
		{Name: "evil", Value: "1", Domain: "douyin.com.evil.test"},
		{Name: "prefix", Value: "1", Domain: "evildouyin.com"},
	})

	if values["kept"] != "1" {
		t.Fatalf("douyinNetworkCookies did not keep valid domain cookie")
	}
	if _, ok := values["evil"]; ok {
		t.Fatalf("douyinNetworkCookies kept suffix-confused domain cookie")
	}
	if _, ok := values["prefix"]; ok {
		t.Fatalf("douyinNetworkCookies kept prefix-confused domain cookie")
	}
}
