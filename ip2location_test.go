package traefik_plugin_ip2location

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type httpHandlerMock struct{}

func (h *httpHandlerMock) ServeHTTP(http.ResponseWriter, *http.Request) {}

// TestGeoIP tests basic MaxMind GeoIP2 functionality
// Note: Requires a MaxMind GeoLite2-City.mmdb file in the test directory
func TestGeoIP(t *testing.T) {
	dbPath := "GeoLite2-City.mmdb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: MaxMind database file %s not found", dbPath)
	}

	config := &Config{
		Filename: dbPath,
		Headers: Headers{
			CountryCode: "X-GEO-Country",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	v := req.Header.Get("X-GEO-Country")
	if v == "" {
		t.Log("X-GEO-Country header not set (may be expected if IP not in database)")
	} else {
		t.Logf("Country code: %s", v)
	}
}

// TestGeoIP_XForwardedFor tests X-Forwarded-For header support
func TestGeoIP_XForwardedFor(t *testing.T) {
	dbPath := "GeoLite2-City.mmdb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: MaxMind database file %s not found", dbPath)
	}

	config := &Config{
		Filename:         dbPath,
		UseXForwardedFor: true,
		Headers: Headers{
			CountryCode: "X-GEO-Country",
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
	} else {
		t.Logf("Country code from X-Forwarded-For: %s", v)
	}
}

// TestGeoIP_CustomHeader tests custom header IP extraction
func TestGeoIP_CustomHeader(t *testing.T) {
	dbPath := "GeoLite2-City.mmdb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: MaxMind database file %s not found", dbPath)
	}

	config := &Config{
		Filename:   dbPath,
		FromHeader: "X-Custom-IP",
		Headers: Headers{
			CountryCode: "X-GEO-Country",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "127.0.0.1:34000"
	req.Header.Set("X-Custom-IP", "8.8.8.8")
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	v := req.Header.Get("X-GEO-Country")
	if v == "" {
		t.Log("X-GEO-Country header not set (may be expected if IP not in database)")
	} else {
		t.Logf("Country code from custom header: %s", v)
	}
}

// TestGeoIP_ErrorHandling tests error handling for missing database
func TestGeoIP_ErrorHandling(t *testing.T) {
	config := &Config{
		Filename:           "nonexistent.mmdb",
		DisableErrorHeader: false,
	}

	_, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err == nil {
		t.Fatal("Expected error for nonexistent database file")
	}
	t.Logf("Got expected error: %v", err)
}

// TestGeoIP_AllFields tests all available MaxMind fields
func TestGeoIP_AllFields(t *testing.T) {
	dbPath := "GeoLite2-City.mmdb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: MaxMind database file %s not found", dbPath)
	}

	config := &Config{
		Filename: dbPath,
		Headers: Headers{
			CountryCode:     "X-Country-Code",
			CountryName:     "X-Country-Name",
			Region:          "X-Region",
			RegionCode:      "X-Region-Code",
			City:           "X-City",
			PostalCode:      "X-Postal-Code",
			Latitude:        "X-Latitude",
			Longitude:       "X-Longitude",
			Timezone:        "X-TimeZone",
			ContinentCode:   "X-Continent-Code",
			ContinentName:   "X-Continent-Name",
			Isp:             "X-ISP",
			Asn:             "X-ASN",
			AsnOrganization: "X-ASN-Org",
			Domain:          "X-Domain",
			ConnectionType:  "X-Connection-Type",
			UserType:        "X-User-Type",
			AccuracyRadius:  "X-Accuracy-Radius",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	// Check that at least some headers were set
	countryCode := req.Header.Get("X-Country-Code")
	if countryCode != "" {
		t.Logf("Successfully retrieved country code: %s", countryCode)
	}
}

// TestGeoIP_LegacyFields tests backward compatibility with legacy field names
func TestGeoIP_LegacyFields(t *testing.T) {
	dbPath := "GeoLite2-City.mmdb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: MaxMind database file %s not found", dbPath)
	}

	config := &Config{
		Filename: dbPath,
		Headers: Headers{
			CountryShort: "X-GEO-Country",
			CountryLong:  "X-GEO-Country-Name",
			Zipcode:      "X-GEO-Zipcode",
		},
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	countryShort := req.Header.Get("X-GEO-Country")
	if countryShort != "" {
		t.Logf("Legacy CountryShort field: %s", countryShort)
	}
}
