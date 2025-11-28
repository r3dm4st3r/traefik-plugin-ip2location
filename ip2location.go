package traefik_plugin_ip2location

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Config the plugin configuration (flattened for Traefik Yaegi compatibility).
type Config struct {
	Filename           string   `json:"filename,omitempty" yaml:"filename,omitempty"`
	FromHeader         string   `json:"from_header,omitempty" yaml:"from_header,omitempty"`
	
	// Header mappings - flattened (no nested struct for Yaegi compatibility)
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
	
	DisableErrorHeader bool     `json:"disable_error_header,omitempty" yaml:"disable_error_header,omitempty"`
	UseXForwardedFor   bool     `json:"use_x_forwarded_for,omitempty" yaml:"use_x_forwarded_for,omitempty"`
	UseXRealIP         bool     `json:"use_x_real_ip,omitempty" yaml:"use_x_real_ip,omitempty"`
	TrustedProxies     []string `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
	Debug              bool     `json:"debug,omitempty" yaml:"debug,omitempty"`
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
	// Header mappings - flattened
	countryCode        string
	countryName         string
	region              string
	regionCode          string
	city                string
	postalCode          string
	latitude            string
	longitude           string
	timezone            string
	continentCode       string
	continentName       string
	isp                 string
	asn                 string
	asnOrganization     string
	domain              string
	connectionType      string
	userType            string
	accuracyRadius      string
	// Legacy fields
	countryShort        string
	countryLong         string
	zipcode             string
	disableErrorHeader  bool
	useXForwardedFor    bool
	useXRealIP          bool
	trustedProxies      []*net.IPNet
	debug               bool
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
		// Header mappings - flattened
		countryCode:        config.CountryCode,
		countryName:        config.CountryName,
		region:             config.Region,
		regionCode:         config.RegionCode,
		city:               config.City,
		postalCode:         config.PostalCode,
		latitude:           config.Latitude,
		longitude:          config.Longitude,
		timezone:           config.Timezone,
		continentCode:      config.ContinentCode,
		continentName:      config.ContinentName,
		isp:                config.Isp,
		asn:                config.Asn,
		asnOrganization:    config.AsnOrganization,
		domain:             config.Domain,
		connectionType:     config.ConnectionType,
		userType:           config.UserType,
		accuracyRadius:     config.AccuracyRadius,
		// Legacy fields
		countryShort:       config.CountryShort,
		countryLong:        config.CountryLong,
		zipcode:            config.Zipcode,
		disableErrorHeader: config.DisableErrorHeader,
		useXForwardedFor:   config.UseXForwardedFor,
		useXRealIP:         config.UseXRealIP,
		debug:              config.Debug,
	}

	// Validate database file exists and is readable
	if config.Debug {
		if fileInfo, err := os.Stat(config.Filename); err == nil {
			log.Printf("[geoip] Database file: %s, Size: %d bytes", config.Filename, fileInfo.Size())
		}
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
		if g.debug {
			log.Printf("[geoip] Error getting IP: %v, RemoteAddr: %s", err, req.RemoteAddr)
		}
		if !g.disableErrorHeader {
			req.Header.Set("X-GEOIP-ERROR", err.Error())
			rw.Header().Set("X-GEOIP-ERROR", err.Error())
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	if ip == nil {
		if g.debug {
			log.Printf("[geoip] Could not determine IP, RemoteAddr: %s", req.RemoteAddr)
		}
		if !g.disableErrorHeader {
			req.Header.Set("X-GEOIP-ERROR", "could not determine client IP")
			rw.Header().Set("X-GEOIP-ERROR", "could not determine client IP")
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	if g.debug {
		log.Printf("[geoip] Looking up IP: %s (from RemoteAddr: %s)", ip.String(), req.RemoteAddr)
	}

	record, err := g.db.LookupIP(ip)
	if err != nil {
		if g.debug {
			log.Printf("[geoip] Lookup failed for IP %s: %v", ip.String(), err)
		}
		if !g.disableErrorHeader {
			errorMsg := fmt.Sprintf("database lookup failed: %v", err)
			req.Header.Set("X-GEOIP-ERROR", errorMsg)
			rw.Header().Set("X-GEOIP-ERROR", errorMsg)
		}
		g.next.ServeHTTP(rw, req)
		return
	}

	if g.debug {
		log.Printf("[geoip] Lookup successful for IP %s: Country=%s, City=%s", 
			ip.String(), record.Country.IsoCode, record.City.Names["en"])
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
	if g.countryCode != "" && record.Country.IsoCode != "" {
		req.Header.Set(g.countryCode, record.Country.IsoCode)
	}
	if g.countryName != "" && record.Country.Names["en"] != "" {
		req.Header.Set(g.countryName, record.Country.Names["en"])
	}
	// Legacy country fields
	if g.countryShort != "" && record.Country.IsoCode != "" {
		req.Header.Set(g.countryShort, record.Country.IsoCode)
	}
	if g.countryLong != "" && record.Country.Names["en"] != "" {
		req.Header.Set(g.countryLong, record.Country.Names["en"])
	}

	// Continent
	if g.continentCode != "" && record.Continent.Code != "" {
		req.Header.Set(g.continentCode, record.Continent.Code)
	}
	if g.continentName != "" && record.Continent.Names["en"] != "" {
		req.Header.Set(g.continentName, record.Continent.Names["en"])
	}

	// Subdivision (Region/State)
	if len(record.Subdivisions) > 0 {
		subdivision := record.Subdivisions[0]
		if g.region != "" && subdivision.Names["en"] != "" {
			req.Header.Set(g.region, subdivision.Names["en"])
		}
		if g.regionCode != "" && subdivision.IsoCode != "" {
			req.Header.Set(g.regionCode, subdivision.IsoCode)
		}
	}

	// City
	if g.city != "" && record.City.Names["en"] != "" {
		req.Header.Set(g.city, record.City.Names["en"])
	}

	// Postal Code
	if g.postalCode != "" && record.Postal.Code != "" {
		req.Header.Set(g.postalCode, record.Postal.Code)
	}
	// Legacy zipcode field
	if g.zipcode != "" && record.Postal.Code != "" {
		req.Header.Set(g.zipcode, record.Postal.Code)
	}

	// Location
	if g.latitude != "" && record.Location.Latitude != 0 {
		req.Header.Set(g.latitude, strconv.FormatFloat(record.Location.Latitude, 'f', 6, 64))
	}
	if g.longitude != "" && record.Location.Longitude != 0 {
		req.Header.Set(g.longitude, strconv.FormatFloat(record.Location.Longitude, 'f', 6, 64))
	}
	if g.timezone != "" && record.Location.TimeZone != "" {
		req.Header.Set(g.timezone, record.Location.TimeZone)
	}
	if g.accuracyRadius != "" && record.Location.AccuracyRadius != 0 {
		req.Header.Set(g.accuracyRadius, strconv.Itoa(int(record.Location.AccuracyRadius)))
	}

	// Traits (ISP, ASN, etc.)
	if g.isp != "" && record.Traits.ISP != "" {
		req.Header.Set(g.isp, record.Traits.ISP)
	}
	if g.asn != "" && record.Traits.AutonomousSystemNumber != 0 {
		req.Header.Set(g.asn, strconv.Itoa(int(record.Traits.AutonomousSystemNumber)))
	}
	if g.asnOrganization != "" && record.Traits.AutonomousSystemOrganization != "" {
		req.Header.Set(g.asnOrganization, record.Traits.AutonomousSystemOrganization)
	}
	if g.domain != "" && record.Traits.Domain != "" {
		req.Header.Set(g.domain, record.Traits.Domain)
	}
	if g.connectionType != "" && record.Traits.ConnectionType != "" {
		req.Header.Set(g.connectionType, record.Traits.ConnectionType)
	}
	if g.userType != "" && record.Traits.UserType != "" {
		req.Header.Set(g.userType, record.Traits.UserType)
	}
}

func (g *GeoIP) addResponseHeaders(rw http.ResponseWriter, record *GeoIP2Record) {
	// Test header - always set to verify plugin is working
	rw.Header().Set("X-GeoIP-Test", "plugin-loaded")
	// Debug: show what config was loaded
	rw.Header().Set("X-GeoIP-Config-Country", g.countryCode)

	// Country
	if g.countryCode != "" && record.Country.IsoCode != "" {
		rw.Header().Set(g.countryCode, record.Country.IsoCode)
	}
	if g.countryName != "" && record.Country.Names["en"] != "" {
		rw.Header().Set(g.countryName, record.Country.Names["en"])
	}
	// Legacy country fields
	if g.countryShort != "" && record.Country.IsoCode != "" {
		rw.Header().Set(g.countryShort, record.Country.IsoCode)
	}
	if g.countryLong != "" && record.Country.Names["en"] != "" {
		rw.Header().Set(g.countryLong, record.Country.Names["en"])
	}

	// Continent
	if g.continentCode != "" && record.Continent.Code != "" {
		rw.Header().Set(g.continentCode, record.Continent.Code)
	}
	if g.continentName != "" && record.Continent.Names["en"] != "" {
		rw.Header().Set(g.continentName, record.Continent.Names["en"])
	}

	// Subdivision (Region/State)
	if len(record.Subdivisions) > 0 {
		subdivision := record.Subdivisions[0]
		if g.region != "" && subdivision.Names["en"] != "" {
			rw.Header().Set(g.region, subdivision.Names["en"])
		}
		if g.regionCode != "" && subdivision.IsoCode != "" {
			rw.Header().Set(g.regionCode, subdivision.IsoCode)
		}
	}

	// City
	if g.city != "" && record.City.Names["en"] != "" {
		rw.Header().Set(g.city, record.City.Names["en"])
	}

	// Postal Code
	if g.postalCode != "" && record.Postal.Code != "" {
		rw.Header().Set(g.postalCode, record.Postal.Code)
	}
	// Legacy zipcode field
	if g.zipcode != "" && record.Postal.Code != "" {
		rw.Header().Set(g.zipcode, record.Postal.Code)
	}

	// Location
	if g.latitude != "" && record.Location.Latitude != 0 {
		rw.Header().Set(g.latitude, strconv.FormatFloat(record.Location.Latitude, 'f', 6, 64))
	}
	if g.longitude != "" && record.Location.Longitude != 0 {
		rw.Header().Set(g.longitude, strconv.FormatFloat(record.Location.Longitude, 'f', 6, 64))
	}
	if g.timezone != "" && record.Location.TimeZone != "" {
		rw.Header().Set(g.timezone, record.Location.TimeZone)
	}
	if g.accuracyRadius != "" && record.Location.AccuracyRadius != 0 {
		rw.Header().Set(g.accuracyRadius, strconv.Itoa(int(record.Location.AccuracyRadius)))
	}

	// Traits (ISP, ASN, etc.)
	if g.isp != "" && record.Traits.ISP != "" {
		rw.Header().Set(g.isp, record.Traits.ISP)
	}
	if g.asn != "" && record.Traits.AutonomousSystemNumber != 0 {
		rw.Header().Set(g.asn, strconv.Itoa(int(record.Traits.AutonomousSystemNumber)))
	}
	if g.asnOrganization != "" && record.Traits.AutonomousSystemOrganization != "" {
		rw.Header().Set(g.asnOrganization, record.Traits.AutonomousSystemOrganization)
	}
	if g.domain != "" && record.Traits.Domain != "" {
		rw.Header().Set(g.domain, record.Traits.Domain)
	}
	if g.connectionType != "" && record.Traits.ConnectionType != "" {
		rw.Header().Set(g.connectionType, record.Traits.ConnectionType)
	}
	if g.userType != "" && record.Traits.UserType != "" {
		rw.Header().Set(g.userType, record.Traits.UserType)
	}
}
