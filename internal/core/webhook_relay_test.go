package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestForwardWebhookPreservesRequest(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery + " sig=" + r.Header.Get("Stripe-Signature") + " body=" + string(body)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	port, _ := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])

	hdr := http.Header{}
	hdr.Set("Stripe-Signature", "t=1,v1=abc")
	hdr.Set("Host", "should-be-dropped")
	forwardWebhook("POST", port, "/hooks/stripe", "id=7", hdr, []byte(`{"evt":1}`))

	select {
	case s := <-got:
		want := "POST /hooks/stripe?id=7 sig=t=1,v1=abc body={\"evt\":1}"
		if s != want {
			t.Fatalf("delivered %q, want %q", s, want)
		}
	default:
		t.Fatal("backend received nothing")
	}
}

func TestResolveSvcKey(t *testing.T) {
	cfg := &config.Config{Repos: map[string]*config.Dir{
		"acme-api": {Alias: "api", Services: map[string]*config.Service{"server": {Cmd: "x"}}},
		"multi":    {Alias: "multi", Services: map[string]*config.Service{"a": {}, "b": {}}},
	}}
	if k, ok := resolveSvcKey(cfg, "api"); !ok || k != "api~server" {
		t.Fatalf("api → %q,%v want api~server", k, ok)
	}
	if k, ok := resolveSvcKey(cfg, "multi/b"); !ok || k != "multi~b" {
		t.Fatalf("multi/b → %q,%v want multi~b", k, ok)
	}
	if _, ok := resolveSvcKey(cfg, "multi"); ok {
		t.Fatal("multi should be ambiguous")
	}
	if _, ok := resolveSvcKey(cfg, "ghost"); ok {
		t.Fatal("unknown repo should fail")
	}
}
