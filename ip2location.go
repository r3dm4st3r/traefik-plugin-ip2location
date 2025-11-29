package traefik_plugin_ip2location

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Config the plugin configuration (flattened for Traefik Yaegi compatibility).
type Config struct {
	Filename           string   `json:"filename,omitempty" yaml:"filename,omitempty"`
	FromHeader         string   `json:"from_header,omitempty" yaml:"from_header,omitempty"`
	ClientIp           string   `json:"client_ip,omitempty" yaml:"client_ip,omitempty"`
	
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
	UseXClientIP       bool     `json:"use_x_client_ip,omitempty" yaml:"use_x_client_ip,omitempty"`
	TrustedProxies     []string `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		UseXForwardedFor: true,
		UseXRealIP:       true,
		UseXClientIP:     true,
	}
}

// GeoIP plugin using IP2Location BIN database (no external dependencies).
type GeoIP struct {
	next               http.Handler
	name               string
	fromHeader         string
	clientIp           string
	db                 *DB
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
	useXClientIP        bool
	trustedProxies      []*net.IPNet
}

// New creates a new GeoIP plugin.
func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	db, err := OpenDB(config.Filename)
	if err != nil {
		return nil, fmt.Errorf("error opening IP2Location database file: %w", err)
	}

	plugin := &GeoIP{
		next:               next,
		name:               name,
		fromHeader:         config.FromHeader,
		clientIp:           config.ClientIp,
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
		useXClientIP:       config.UseXClientIP,
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

	record, err := g.db.Get_all(ip.String())
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
	g.addHeaders(req, ip, record)

	// Also add headers to response (for client)
	g.addResponseHeaders(rw, ip, record)

	g.next.ServeHTTP(rw, req)
}

// getIP extracts the client IP address from the request.
// Priority order:
// 1. Custom header (if configured)
// 2. X-Real-IP (if enabled and trusted)
// 3. X-Client-IP (if enabled and trusted)
// 4. X-Forwarded-For (if enabled and trusted)
// 5. RemoteAddr
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

	// Priority 3: X-Client-IP
	if g.useXClientIP && trustProxy {
		ipStr := req.Header.Get("X-Client-IP")
		if ipStr != "" {
			ip := g.parseIP(ipStr)
			if ip != nil {
				return ip, nil
			}
		}
	}

	// Priority 4: X-Forwarded-For
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

func (g *GeoIP) addHeaders(req *http.Request, ip net.IP, record IP2Locationrecord) {
	// Add client IP header
	if g.clientIp != "" && ip != nil {
		req.Header.Set(g.clientIp, ip.String())
	}

	// Country
	if g.countryCode != "" && record.Country_short != "" {
		req.Header.Set(g.countryCode, record.Country_short)
	}
	if g.countryName != "" && record.Country_long != "" {
		req.Header.Set(g.countryName, record.Country_long)
	}
	// Legacy country fields
	if g.countryShort != "" && record.Country_short != "" {
		req.Header.Set(g.countryShort, record.Country_short)
	}
	if g.countryLong != "" && record.Country_long != "" {
		req.Header.Set(g.countryLong, record.Country_long)
	}

	// Region
	if g.region != "" && record.Region != "" {
		req.Header.Set(g.region, record.Region)
	}
	// Note: IP2Location doesn't have region code, so regionCode won't be set

	// City
	if g.city != "" && record.City != "" {
		req.Header.Set(g.city, record.City)
	}

	// Postal Code
	if g.postalCode != "" && record.Zipcode != "" {
		req.Header.Set(g.postalCode, record.Zipcode)
	}
	// Legacy zipcode field
	if g.zipcode != "" && record.Zipcode != "" {
		req.Header.Set(g.zipcode, record.Zipcode)
	}

	// Location
	if g.latitude != "" && record.Latitude != 0 {
		req.Header.Set(g.latitude, strconv.FormatFloat(float64(record.Latitude), 'f', 6, 32))
	}
	if g.longitude != "" && record.Longitude != 0 {
		req.Header.Set(g.longitude, strconv.FormatFloat(float64(record.Longitude), 'f', 6, 32))
	}
	if g.timezone != "" && record.Timezone != "" {
		req.Header.Set(g.timezone, record.Timezone)
	}
	// Note: IP2Location doesn't have accuracy radius

	// ISP, Domain
	if g.isp != "" && record.Isp != "" {
		req.Header.Set(g.isp, record.Isp)
	}
	if g.domain != "" && record.Domain != "" {
		req.Header.Set(g.domain, record.Domain)
	}
	// Note: IP2Location doesn't have ASN, ConnectionType, UserType
}

func (g *GeoIP) addResponseHeaders(rw http.ResponseWriter, ip net.IP, record IP2Locationrecord) {
	// Add client IP header
	if g.clientIp != "" && ip != nil {
		rw.Header().Set(g.clientIp, ip.String())
	}

	// Country
	if g.countryCode != "" && record.Country_short != "" {
		rw.Header().Set(g.countryCode, record.Country_short)
	}
	if g.countryName != "" && record.Country_long != "" {
		rw.Header().Set(g.countryName, record.Country_long)
	}
	// Legacy country fields
	if g.countryShort != "" && record.Country_short != "" {
		rw.Header().Set(g.countryShort, record.Country_short)
	}
	if g.countryLong != "" && record.Country_long != "" {
		rw.Header().Set(g.countryLong, record.Country_long)
	}

	// Region
	if g.region != "" && record.Region != "" {
		rw.Header().Set(g.region, record.Region)
	}
	// Note: IP2Location doesn't have region code

	// City
	if g.city != "" && record.City != "" {
		rw.Header().Set(g.city, record.City)
	}

	// Postal Code
	if g.postalCode != "" && record.Zipcode != "" {
		rw.Header().Set(g.postalCode, record.Zipcode)
	}
	// Legacy zipcode field
	if g.zipcode != "" && record.Zipcode != "" {
		rw.Header().Set(g.zipcode, record.Zipcode)
	}

	// Location
	if g.latitude != "" && record.Latitude != 0 {
		rw.Header().Set(g.latitude, strconv.FormatFloat(float64(record.Latitude), 'f', 6, 32))
	}
	if g.longitude != "" && record.Longitude != 0 {
		rw.Header().Set(g.longitude, strconv.FormatFloat(float64(record.Longitude), 'f', 6, 32))
	}
	if g.timezone != "" && record.Timezone != "" {
		rw.Header().Set(g.timezone, record.Timezone)
	}
	// Note: IP2Location doesn't have accuracy radius

	// ISP, Domain
	if g.isp != "" && record.Isp != "" {
		rw.Header().Set(g.isp, record.Isp)
	}
	if g.domain != "" && record.Domain != "" {
		rw.Header().Set(g.domain, record.Domain)
	}
	// Note: IP2Location doesn't have ASN, ConnectionType, UserType
}
