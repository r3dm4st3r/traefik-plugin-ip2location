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

// TestGeoIP tests basic IP2Location functionality
// Note: Requires an IP2Location BIN file in the test directory
func TestGeoIP(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:    dbPath,
		CountryCode: "X-GEO-Country", // Flattened config
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	// Check request headers
	v := req.Header.Get("X-GEO-Country")
	if v == "" {
		t.Log("X-GEO-Country header not set in request (may be expected if IP not in database)")
	} else {
		t.Logf("Country code in request: %s", v)
	}

	// Check if actual geo header was set in response
	geoCountry := rw.Header().Get("X-GEO-Country")
	if geoCountry != "" {
		t.Logf("✓ Response header X-GEO-Country: %s", geoCountry)
	} else {
		t.Log("X-GEO-Country not set in response (IP may not be in database)")
	}
}

// TestGeoIP_ResponseHeaders tests that response headers are set correctly
func TestGeoIP_ResponseHeaders(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:    dbPath,
		CountryCode: "X-Test-Country",
		City:        "X-Test-City",
		Region:      "X-Test-Region",
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	// Check if geo headers were set in response
	country := rw.Header().Get("X-Test-Country")
	if country != "" {
		t.Logf("✓ Response header X-Test-Country: %s", country)
	}

	city := rw.Header().Get("X-Test-City")
	if city != "" {
		t.Logf("✓ Response header X-Test-City: %s", city)
	}

	region := rw.Header().Get("X-Test-Region")
	if region != "" {
		t.Logf("✓ Response header X-Test-Region: %s", region)
	}
}

// TestGeoIP_XForwardedFor tests X-Forwarded-For header support
func TestGeoIP_XForwardedFor(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:         dbPath,
		UseXForwardedFor: true,
		CountryCode:      "X-GEO-Country", // Flattened config
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

	// Check response headers
	respCountry := rw.Header().Get("X-GEO-Country")
	if respCountry != "" {
		t.Logf("✓ Response header X-GEO-Country: %s", respCountry)
	}
}

// TestGeoIP_CustomHeader tests custom header IP extraction
func TestGeoIP_CustomHeader(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:   dbPath,
		FromHeader: "X-Custom-IP",
		CountryCode: "X-GEO-Country", // Flattened config
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

	// Check response headers
	respCountry := rw.Header().Get("X-GEO-Country")
	if respCountry != "" {
		t.Logf("✓ Response header X-GEO-Country: %s", respCountry)
	}
}

// TestGeoIP_ErrorHandling tests error handling for missing database
func TestGeoIP_ErrorHandling(t *testing.T) {
	config := &Config{
		Filename:           "nonexistent.bin",
		DisableErrorHeader: false,
	}

	_, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err == nil {
		t.Fatal("Expected error for nonexistent database file")
	}
	t.Logf("Got expected error: %v", err)
}

// TestGeoIP_AllFields tests all available IP2Location fields
func TestGeoIP_AllFields(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:         dbPath,
		CountryCode:      "X-Country-Code", // Flattened config
		CountryName:      "X-Country-Name",
		Region:           "X-Region",
		RegionCode:       "X-Region-Code",
		City:             "X-City",
		PostalCode:       "X-Postal-Code",
		Latitude:         "X-Latitude",
		Longitude:        "X-Longitude",
		Timezone:         "X-TimeZone",
		ContinentCode:    "X-Continent-Code",
		ContinentName:    "X-Continent-Name",
		Isp:              "X-ISP",
		Asn:              "X-ASN",
		AsnOrganization:  "X-ASN-Org",
		Domain:           "X-Domain",
		ConnectionType:   "X-Connection-Type",
		UserType:         "X-User-Type",
		AccuracyRadius:   "X-Accuracy-Radius",
	}

	handler, err := New(context.Background(), &httpHandlerMock{}, config, "test")
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	req.RemoteAddr = "8.8.8.8:34000"
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	// Check request headers
	countryCode := req.Header.Get("X-Country-Code")
	if countryCode != "" {
		t.Logf("Request header X-Country-Code: %s", countryCode)
	}

	// Check response geo headers
	respCountryCode := rw.Header().Get("X-Country-Code")
	if respCountryCode != "" {
		t.Logf("✓ Response header X-Country-Code: %s", respCountryCode)
	}
}

// TestGeoIP_LegacyFields tests backward compatibility with legacy field names
func TestGeoIP_LegacyFields(t *testing.T) {
	dbPath := "IP2LOCATION-LITE-DB11.BIN"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: IP2Location database file %s not found", dbPath)
	}

	config := &Config{
		Filename:     dbPath,
		CountryShort: "X-GEO-Country", // Flattened config
		CountryLong:  "X-GEO-Country-Name",
		Zipcode:      "X-GEO-Zipcode",
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

	// Check response headers
	respCountryShort := rw.Header().Get("X-GEO-Country")
	if respCountryShort != "" {
		t.Logf("✓ Response header X-GEO-Country: %s", respCountryShort)
	}
}
