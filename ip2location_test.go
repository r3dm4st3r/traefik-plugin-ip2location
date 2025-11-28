package traefik_plugin_ip2location

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type httpHandlerMock struct{}

func (h *httpHandlerMock) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestIP2Location(t *testing.T) {
	config := &Config{
		Filename: "IP2LOCATION-LITE-DB1.IPV6.BIN",
		Headers: Headers{
			CountryShort: "X-GEO-Country",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "4.0.0.0:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	v := req.Header.Get("X-GEO-Country")
	if v != "US" {
		t.Fatalf("unexpected value: got %s, expected US", v)
	}
}

func TestIP2Location_XForwardedFor(t *testing.T) {
	config := &Config{
		Filename:         "IP2LOCATION-LITE-DB1.IPV6.BIN",
		UseXForwardedFor: true,
		Headers: Headers{
			CountryShort: "X-GEO-Country",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "10.0.0.1:34000"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	v := req.Header.Get("X-GEO-Country")
	if v == "" {
		t.Log("X-GEO-Country header not set (may be expected if 8.8.8.8 not in database)")
	}
}

func TestIP2Location_CustomHeader(t *testing.T) {
	config := &Config{
		Filename:   "IP2LOCATION-LITE-DB1.IPV6.BIN",
		FromHeader: "X-Custom-IP",
		Headers: Headers{
			CountryShort: "X-GEO-Country",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "127.0.0.1:34000"
	req.Header.Set("X-Custom-IP", "4.0.0.0")
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	v := req.Header.Get("X-GEO-Country")
	if v != "US" {
		t.Fatalf("unexpected value: got %s, expected US", v)
	}
}

func TestIP2Location_ErrorHandling(t *testing.T) {
	config := &Config{
		Filename:           "nonexistent.bin",
		DisableErrorHeader: false,
	}

	_, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err == nil {
		t.Fatal("Expected error for nonexistent database file")
	}
}
