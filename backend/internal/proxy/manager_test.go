package proxy

import "testing"

func TestParseProxyList(t *testing.T) {
	raw := "\n127.0.0.1:8080\nhttp://127.0.0.1:8081\n \n"
	list := ParseProxyList(raw)
	if len(list) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(list))
	}
	if list[0] != "http://127.0.0.1:8080" {
		t.Fatalf("expected normalized proxy, got %s", list[0])
	}
}
