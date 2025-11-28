package traefik_plugin_ip2location

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Headers part of the configuration
type Headers struct {
	CountryCode     string `json:"country_code,omitempty" yaml:"country_code,omitempty"`
	CountryName     string `json:"country_name,omitempty" yaml:"country_name,omitempty"`
	Region          string `json:"region,omitempty" yaml:"region,omitempty"`
	RegionCode      string `json:"region_code,omitempty" yaml:"region_code,omitempty"`
	City            string `json:"city,omitempty" yaml:"city,omitempty"`
	PostalCode      string `json:"postal_code,omitempty" yaml:"postal_code,omitempty"`
	Latitude        string `json:"latitude,omitempty" yaml:"latitude,omitempty"`
	Longitude       string `json:"longitude,omitempty" yaml:"longitude,omitempty"`
	Timezone        string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	ContinentCode   string `json:"continent_code,omitempty" yaml:"continent_code,omitempty"`
	ContinentName   string `json:"continent_name,omitempty" yaml:"continent_name,omitempty"`
	Isp             string `json:"isp,omitempty" yaml:"isp,omitempty"`
	Asn             string `json:"asn,omitempty" yaml:"asn,omitempty"`
	AsnOrganization string `json:"asn_organization,omitempty" yaml:"asn_organization,omitempty"`
	Domain          string `json:"domain,omitempty" yaml:"domain,omitempty"`
	ConnectionType  string `json:"connection_type,omitempty" yaml:"connection_type,omitempty"`
	UserType        string `json:"user_type,omitempty" yaml:"user_type,omitempty"`
	AccuracyRadius  string `json:"accuracy_radius,omitempty" yaml:"accuracy_radius,omitempty"`
	// Legacy fields for backward compatibility
	CountryShort string `json:"country_short,omitempty" yaml:"country_short,omitempty"`
	CountryLong  string `json:"country_long,omitempty" yaml:"country_long,omitempty"`
	Zipcode      string `json:"zipcode,omitempty" yaml:"zipcode,omitempty"`
}

// Config the plugin configuration.
type Config struct {
	Filename           string   `json:"filename,omitempty" yaml:"filename,omitempty"`
	FromHeader         string   `json:"from_header,omitempty" yaml:"from_header,omitempty"`
	Headers            Headers  `json:"headers,omitempty" yaml:"headers,omitempty"`
	DisableErrorHeader bool     `json:"disable_error_header,omitempty" yaml:"disable_error_header,omitempty"`
	UseXForwardedFor   bool     `json:"use_x_forwarded_for,omitempty" yaml:"use_x_forwarded_for,omitempty"`
	UseXRealIP         bool     `json:"use_x_real_ip,omitempty" yaml:"use_x_real_ip,omitempty"`
	TrustedProxies     []string `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		UseXForwardedFor: true,
		UseXRealIP:       true,
	}
}

// GeoIP plugin using MaxMind GeoIP2 database (no external dependencies).
type GeoIP struct {
	next               http.Handler
	name               string
	fromHeader         string
	db                 *MMDBReader
	headers            Headers
	disableErrorHeader bool
	useXForwardedFor   bool
	useXRealIP         bool
	trustedProxies     []*net.IPNet
}

// New creates a new GeoIP plugin.
func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	db, err := OpenMMDB(config.Filename)
	if err != nil {
		return nil, fmt.Errorf("error opening MaxMind database file: %w", err)
	}

	plugin := &GeoIP{
		next:               next,
		name:               name,
		fromHeader:         config.FromHeader,
		db:                 db,
		headers:            config.Headers,
		disableErrorHeader: config.DisableErrorHeader,
		useXForwardedFor:   config.UseXForwardedFor,
		useXRealIP:         config.UseXRealIP,
	}

	// Parse trusted proxy CIDR ranges
	if len(config.TrustedProxies) > 0 {
		plugin.trustedProxies = make([]*net.IPNet, 0, len(config.TrustedProxies))
		for _, proxy := range config.TrustedProxies {
			_, ipNet, err := net.ParseCIDR(proxy)
			if err != nil {
				// Try parsing as single IP
				ip := net.ParseIP(proxy)
				if ip == nil {
					log.Printf("[geoip] Warning: invalid trusted proxy '%s', ignoring", proxy)
					continue
				}
				// Create a /32 or /128 CIDR for single IP
				if ip.To4() != nil {
					_, ipNet, _ = net.ParseCIDR(ip.String() + "/32")
				} else {
					_, ipNet, _ = net.ParseCIDR(ip.String() + "/128")
				}
			}
			plugin.trustedProxies = append(plugin.trustedProxies, ipNet)
		}
	}

	return plugin, nil
}

func (g *GeoIP) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ip, err := g.getIP(req)
	if err != nil {
		if !g.disableErrorHeader {
			req.Header.Set("X-GEOIP-ERROR", err.Error())
			rw.Header().Set("X-GEOIP-ERROR", err.Error())
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	if ip == nil {
		if !g.disableErrorHeader {
			req.Header.Set("X-GEOIP-ERROR", "could not determine client IP")
			rw.Header().Set("X-GEOIP-ERROR", "could not determine client IP")
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	record, err := g.db.LookupIP(ip)
	if err != nil {
		if !g.disableErrorHeader {
			errorMsg := fmt.Sprintf("database lookup failed: %v", err)
			req.Header.Set("X-GEOIP-ERROR", errorMsg)
			rw.Header().Set("X-GEOIP-ERROR", errorMsg)
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	// Add headers to request (for backend services)
	g.addHeaders(req, record)

	// Also add headers to response (for client)
	g.addResponseHeaders(rw, record)

	g.next.ServeHTTP(rw, req)
}

// getIP extracts the client IP address from the request.
// Priority order:
// 1. Custom header (if configured)
// 2. X-Real-IP (if enabled and trusted)
// 3. X-Forwarded-For (if enabled and trusted)
// 4. RemoteAddr
func (g *GeoIP) getIP(req *http.Request) (net.IP, error) {
	// Priority 1: Custom header
	if g.fromHeader != "" {
		ipStr := req.Header.Get(g.fromHeader)
		if ipStr != "" {
			ip := g.parseIP(ipStr)
			if ip != nil {
				return ip, nil
			}
		}
	}

	// Check if we should trust proxy headers
	trustProxy := g.isTrustedProxy(req.RemoteAddr)

	// Priority 2: X-Real-IP
	if g.useXRealIP && trustProxy {
		ipStr := req.Header.Get("X-Real-IP")
		if ipStr != "" {
			ip := g.parseIP(ipStr)
			if ip != nil {
				return ip, nil
			}
		}
	}

	// Priority 3: X-Forwarded-For
	if g.useXForwardedFor && trustProxy {
		xff := req.Header.Get("X-Forwarded-For")
		if xff != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ipStr := strings.TrimSpace(ips[0])
				ip := g.parseIP(ipStr)
				if ip != nil {
					return ip, nil
				}
			}
		}
	}

	// Priority 4: RemoteAddr
	addr, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RemoteAddr: %w", err)
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address in RemoteAddr: %s", addr)
	}

	return ip, nil
}

// parseIP parses an IP address string, handling both IPv4 and IPv6
func (g *GeoIP) parseIP(ipStr string) net.IP {
	ipStr = strings.TrimSpace(ipStr)
	// Remove port if present
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	return net.ParseIP(ipStr)
}

// isTrustedProxy checks if the request comes from a trusted proxy
func (g *GeoIP) isTrustedProxy(remoteAddr string) bool {
	// If no trusted proxies configured, trust all (backward compatible)
	if len(g.trustedProxies) == 0 {
		return true
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, trustedNet := range g.trustedProxies {
		if trustedNet.Contains(ip) {
			return true
		}
	}

	return false
}

func (g *GeoIP) addHeaders(req *http.Request, record *GeoIP2Record) {
	// Country
	if g.headers.CountryCode != "" && record.Country.IsoCode != "" {
		req.Header.Set(g.headers.CountryCode, record.Country.IsoCode)
	}
	if g.headers.CountryName != "" && record.Country.Names["en"] != "" {
		req.Header.Set(g.headers.CountryName, record.Country.Names["en"])
	}
	// Legacy country fields
	if g.headers.CountryShort != "" && record.Country.IsoCode != "" {
		req.Header.Set(g.headers.CountryShort, record.Country.IsoCode)
	}
	if g.headers.CountryLong != "" && record.Country.Names["en"] != "" {
		req.Header.Set(g.headers.CountryLong, record.Country.Names["en"])
	}

	// Continent
	if g.headers.ContinentCode != "" && record.Continent.Code != "" {
		req.Header.Set(g.headers.ContinentCode, record.Continent.Code)
	}
	if g.headers.ContinentName != "" && record.Continent.Names["en"] != "" {
		req.Header.Set(g.headers.ContinentName, record.Continent.Names["en"])
	}

	// Subdivision (Region/State)
	if len(record.Subdivisions) > 0 {
		subdivision := record.Subdivisions[0]
		if g.headers.Region != "" && subdivision.Names["en"] != "" {
			req.Header.Set(g.headers.Region, subdivision.Names["en"])
		}
		if g.headers.RegionCode != "" && subdivision.IsoCode != "" {
			req.Header.Set(g.headers.RegionCode, subdivision.IsoCode)
		}
	}

	// City
	if g.headers.City != "" && record.City.Names["en"] != "" {
		req.Header.Set(g.headers.City, record.City.Names["en"])
	}

	// Postal Code
	if g.headers.PostalCode != "" && record.Postal.Code != "" {
		req.Header.Set(g.headers.PostalCode, record.Postal.Code)
	}
	// Legacy zipcode field
	if g.headers.Zipcode != "" && record.Postal.Code != "" {
		req.Header.Set(g.headers.Zipcode, record.Postal.Code)
	}

	// Location
	if g.headers.Latitude != "" && record.Location.Latitude != 0 {
		req.Header.Set(g.headers.Latitude, strconv.FormatFloat(record.Location.Latitude, 'f', 6, 64))
	}
	if g.headers.Longitude != "" && record.Location.Longitude != 0 {
		req.Header.Set(g.headers.Longitude, strconv.FormatFloat(record.Location.Longitude, 'f', 6, 64))
	}
	if g.headers.Timezone != "" && record.Location.TimeZone != "" {
		req.Header.Set(g.headers.Timezone, record.Location.TimeZone)
	}
	if g.headers.AccuracyRadius != "" && record.Location.AccuracyRadius != 0 {
		req.Header.Set(g.headers.AccuracyRadius, strconv.Itoa(int(record.Location.AccuracyRadius)))
	}

	// Traits (ISP, ASN, etc.)
	if g.headers.Isp != "" && record.Traits.ISP != "" {
		req.Header.Set(g.headers.Isp, record.Traits.ISP)
	}
	if g.headers.Asn != "" && record.Traits.AutonomousSystemNumber != 0 {
		req.Header.Set(g.headers.Asn, strconv.Itoa(int(record.Traits.AutonomousSystemNumber)))
	}
	if g.headers.AsnOrganization != "" && record.Traits.AutonomousSystemOrganization != "" {
		req.Header.Set(g.headers.AsnOrganization, record.Traits.AutonomousSystemOrganization)
	}
	if g.headers.Domain != "" && record.Traits.Domain != "" {
		req.Header.Set(g.headers.Domain, record.Traits.Domain)
	}
	if g.headers.ConnectionType != "" && record.Traits.ConnectionType != "" {
		req.Header.Set(g.headers.ConnectionType, record.Traits.ConnectionType)
	}
	if g.headers.UserType != "" && record.Traits.UserType != "" {
		req.Header.Set(g.headers.UserType, record.Traits.UserType)
	}
}

func (g *GeoIP) addResponseHeaders(rw http.ResponseWriter, record *GeoIP2Record) {
	// Country
	if g.headers.CountryCode != "" && record.Country.IsoCode != "" {
		rw.Header().Set(g.headers.CountryCode, record.Country.IsoCode)
	}
	if g.headers.CountryName != "" && record.Country.Names["en"] != "" {
		rw.Header().Set(g.headers.CountryName, record.Country.Names["en"])
	}
	// Legacy country fields
	if g.headers.CountryShort != "" && record.Country.IsoCode != "" {
		rw.Header().Set(g.headers.CountryShort, record.Country.IsoCode)
	}
	if g.headers.CountryLong != "" && record.Country.Names["en"] != "" {
		rw.Header().Set(g.headers.CountryLong, record.Country.Names["en"])
	}

	// Continent
	if g.headers.ContinentCode != "" && record.Continent.Code != "" {
		rw.Header().Set(g.headers.ContinentCode, record.Continent.Code)
	}
	if g.headers.ContinentName != "" && record.Continent.Names["en"] != "" {
		rw.Header().Set(g.headers.ContinentName, record.Continent.Names["en"])
	}

	// Subdivision (Region/State)
	if len(record.Subdivisions) > 0 {
		subdivision := record.Subdivisions[0]
		if g.headers.Region != "" && subdivision.Names["en"] != "" {
			rw.Header().Set(g.headers.Region, subdivision.Names["en"])
		}
		if g.headers.RegionCode != "" && subdivision.IsoCode != "" {
			rw.Header().Set(g.headers.RegionCode, subdivision.IsoCode)
		}
	}

	// City
	if g.headers.City != "" && record.City.Names["en"] != "" {
		rw.Header().Set(g.headers.City, record.City.Names["en"])
	}

	// Postal Code
	if g.headers.PostalCode != "" && record.Postal.Code != "" {
		rw.Header().Set(g.headers.PostalCode, record.Postal.Code)
	}
	// Legacy zipcode field
	if g.headers.Zipcode != "" && record.Postal.Code != "" {
		rw.Header().Set(g.headers.Zipcode, record.Postal.Code)
	}

	// Location
	if g.headers.Latitude != "" && record.Location.Latitude != 0 {
		rw.Header().Set(g.headers.Latitude, strconv.FormatFloat(record.Location.Latitude, 'f', 6, 64))
	}
	if g.headers.Longitude != "" && record.Location.Longitude != 0 {
		rw.Header().Set(g.headers.Longitude, strconv.FormatFloat(record.Location.Longitude, 'f', 6, 64))
	}
	if g.headers.Timezone != "" && record.Location.TimeZone != "" {
		rw.Header().Set(g.headers.Timezone, record.Location.TimeZone)
	}
	if g.headers.AccuracyRadius != "" && record.Location.AccuracyRadius != 0 {
		rw.Header().Set(g.headers.AccuracyRadius, strconv.Itoa(int(record.Location.AccuracyRadius)))
	}

	// Traits (ISP, ASN, etc.)
	if g.headers.Isp != "" && record.Traits.ISP != "" {
		rw.Header().Set(g.headers.Isp, record.Traits.ISP)
	}
	if g.headers.Asn != "" && record.Traits.AutonomousSystemNumber != 0 {
		rw.Header().Set(g.headers.Asn, strconv.Itoa(int(record.Traits.AutonomousSystemNumber)))
	}
	if g.headers.AsnOrganization != "" && record.Traits.AutonomousSystemOrganization != "" {
		rw.Header().Set(g.headers.AsnOrganization, record.Traits.AutonomousSystemOrganization)
	}
	if g.headers.Domain != "" && record.Traits.Domain != "" {
		rw.Header().Set(g.headers.Domain, record.Traits.Domain)
	}
	if g.headers.ConnectionType != "" && record.Traits.ConnectionType != "" {
		rw.Header().Set(g.headers.ConnectionType, record.Traits.ConnectionType)
	}
	if g.headers.UserType != "" && record.Traits.UserType != "" {
		rw.Header().Set(g.headers.UserType, record.Traits.UserType)
	}
}
