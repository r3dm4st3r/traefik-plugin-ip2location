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
	CountryShort       string `json:"country_short,omitempty" yaml:"country_short,omitempty"`
	CountryLong        string `json:"country_long,omitempty" yaml:"country_long,omitempty"`
	Region             string `json:"region,omitempty" yaml:"region,omitempty"`
	City               string `json:"city,omitempty" yaml:"city,omitempty"`
	Isp                string `json:"isp,omitempty" yaml:"isp,omitempty"`
	Latitude           string `json:"latitude,omitempty" yaml:"latitude,omitempty"`
	Longitude          string `json:"longitude,omitempty" yaml:"longitude,omitempty"`
	Domain             string `json:"domain,omitempty" yaml:"domain,omitempty"`
	Zipcode            string `json:"zipcode,omitempty" yaml:"zipcode,omitempty"`
	Timezone           string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	Netspeed           string `json:"netspeed,omitempty" yaml:"netspeed,omitempty"`
	Iddcode            string `json:"iddcode,omitempty" yaml:"iddcode,omitempty"`
	Areacode           string `json:"areacode,omitempty" yaml:"areacode,omitempty"`
	Weatherstationcode string `json:"weatherstationcode,omitempty" yaml:"weatherstationcode,omitempty"`
	Weatherstationname string `json:"weatherstationname,omitempty" yaml:"weatherstationname,omitempty"`
	Mcc                string `json:"mcc,omitempty" yaml:"mcc,omitempty"`
	Mnc                string `json:"mnc,omitempty" yaml:"mnc,omitempty"`
	Mobilebrand        string `json:"mobilebrand,omitempty" yaml:"mobilebrand,omitempty"`
	Elevation          string `json:"elevation,omitempty" yaml:"elevation,omitempty"`
	Usagetype          string `json:"usagetype,omitempty" yaml:"usagetype,omitempty"`
}

// Config the plugin configuration.
type Config struct {
	Filename           string  `json:"filename,omitempty" yaml:"filename,omitempty"`
	FromHeader         string  `json:"from_header,omitempty" yaml:"from_header,omitempty"`
	Headers            Headers `json:"headers,omitempty" yaml:"headers,omitempty"`
	DisableErrorHeader bool    `json:"disable_error_header,omitempty" yaml:"disable_error_header,omitempty"`
	UseXForwardedFor   bool    `json:"use_x_forwarded_for,omitempty" yaml:"use_x_forwarded_for,omitempty"`
	UseXRealIP         bool    `json:"use_x_real_ip,omitempty" yaml:"use_x_real_ip,omitempty"`
	TrustedProxies     []string `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		UseXForwardedFor: true,
		UseXRealIP:       true,
	}
}

// IP2Location plugin.
type IP2Location struct {
	next               http.Handler
	name               string
	fromHeader         string
	db                 *DB
	headers            Headers
	disableErrorHeader bool
	useXForwardedFor   bool
	useXRealIP         bool
	trustedProxies     []*net.IPNet
}

// New created a new IP2Location plugin.
func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	db, err := OpenDB(config.Filename)
	if err != nil {
		return nil, fmt.Errorf("error opening database file: %w", err)
	}

	plugin := &IP2Location{
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
					log.Printf("[ip2location] Warning: invalid trusted proxy '%s', ignoring", proxy)
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

func (a *IP2Location) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ip, err := a.getIP(req)
	if err != nil {
		if !a.disableErrorHeader {
			req.Header.Set("X-IP2LOCATION-ERROR", err.Error())
		}
		a.next.ServeHTTP(rw, req)
		return
	}

	if ip == nil {
		if !a.disableErrorHeader {
			req.Header.Set("X-IP2LOCATION-ERROR", "could not determine client IP")
		}
		a.next.ServeHTTP(rw, req)
		return
	}

	record, err := a.db.Get_all(ip.String())
	if err != nil {
		if !a.disableErrorHeader {
			req.Header.Set("X-IP2LOCATION-ERROR", err.Error())
		}
		a.next.ServeHTTP(rw, req)
		return
	}

	a.addHeaders(req, &record)

	a.next.ServeHTTP(rw, req)
}

// getIP extracts the client IP address from the request.
// Priority order:
// 1. Custom header (if configured)
// 2. X-Real-IP (if enabled and trusted)
// 3. X-Forwarded-For (if enabled and trusted)
// 4. RemoteAddr
func (a *IP2Location) getIP(req *http.Request) (net.IP, error) {
	// Priority 1: Custom header
	if a.fromHeader != "" {
		ipStr := req.Header.Get(a.fromHeader)
		if ipStr != "" {
			ip := a.parseIP(ipStr)
			if ip != nil {
				return ip, nil
			}
		}
	}

	// Check if we should trust proxy headers
	trustProxy := a.isTrustedProxy(req.RemoteAddr)

	// Priority 2: X-Real-IP
	if a.useXRealIP && trustProxy {
		ipStr := req.Header.Get("X-Real-IP")
		if ipStr != "" {
			ip := a.parseIP(ipStr)
			if ip != nil {
				return ip, nil
			}
		}
	}

	// Priority 3: X-Forwarded-For
	if a.useXForwardedFor && trustProxy {
		xff := req.Header.Get("X-Forwarded-For")
		if xff != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ipStr := strings.TrimSpace(ips[0])
				ip := a.parseIP(ipStr)
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
func (a *IP2Location) parseIP(ipStr string) net.IP {
	ipStr = strings.TrimSpace(ipStr)
	// Remove port if present
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	return net.ParseIP(ipStr)
}

// isTrustedProxy checks if the request comes from a trusted proxy
func (a *IP2Location) isTrustedProxy(remoteAddr string) bool {
	// If no trusted proxies configured, trust all (backward compatible)
	if len(a.trustedProxies) == 0 {
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

	for _, trustedNet := range a.trustedProxies {
		if trustedNet.Contains(ip) {
			return true
		}
	}

	return false
}

func (a *IP2Location) addHeaders(req *http.Request, record *IP2Locationrecord) {
	if a.headers.CountryShort != "" && record.Country_short != "" {
		req.Header.Set(a.headers.CountryShort, record.Country_short)
	}
	if a.headers.CountryLong != "" && record.Country_long != "" {
		req.Header.Set(a.headers.CountryLong, record.Country_long)
	}
	if a.headers.Region != "" && record.Region != "" {
		req.Header.Set(a.headers.Region, record.Region)
	}
	if a.headers.City != "" && record.City != "" {
		req.Header.Set(a.headers.City, record.City)
	}
	if a.headers.Isp != "" && record.Isp != "" {
		req.Header.Set(a.headers.Isp, record.Isp)
	}
	if a.headers.Latitude != "" && record.Latitude != 0 {
		req.Header.Set(a.headers.Latitude, strconv.FormatFloat(float64(record.Latitude), 'f', 6, 32))
	}
	if a.headers.Longitude != "" && record.Longitude != 0 {
		req.Header.Set(a.headers.Longitude, strconv.FormatFloat(float64(record.Longitude), 'f', 6, 32))
	}
	if a.headers.Domain != "" && record.Domain != "" {
		req.Header.Set(a.headers.Domain, record.Domain)
	}
	if a.headers.Zipcode != "" && record.Zipcode != "" {
		req.Header.Set(a.headers.Zipcode, record.Zipcode)
	}
	if a.headers.Timezone != "" && record.Timezone != "" {
		req.Header.Set(a.headers.Timezone, record.Timezone)
	}
	if a.headers.Netspeed != "" && record.Netspeed != "" {
		req.Header.Set(a.headers.Netspeed, record.Netspeed)
	}
	if a.headers.Iddcode != "" && record.Iddcode != "" {
		req.Header.Set(a.headers.Iddcode, record.Iddcode)
	}
	if a.headers.Areacode != "" && record.Areacode != "" {
		req.Header.Set(a.headers.Areacode, record.Areacode)
	}
	if a.headers.Weatherstationcode != "" && record.Weatherstationcode != "" {
		req.Header.Set(a.headers.Weatherstationcode, record.Weatherstationcode)
	}
	if a.headers.Weatherstationname != "" && record.Weatherstationname != "" {
		req.Header.Set(a.headers.Weatherstationname, record.Weatherstationname)
	}
	if a.headers.Mcc != "" && record.Mcc != "" {
		req.Header.Set(a.headers.Mcc, record.Mcc)
	}
	if a.headers.Mnc != "" && record.Mnc != "" {
		req.Header.Set(a.headers.Mnc, record.Mnc)
	}
	if a.headers.Mobilebrand != "" && record.Mobilebrand != "" {
		req.Header.Set(a.headers.Mobilebrand, record.Mobilebrand)
	}
	if a.headers.Elevation != "" && record.Elevation != 0 {
		req.Header.Set(a.headers.Elevation, strconv.FormatFloat(float64(record.Elevation), 'f', 2, 32))
	}
	if a.headers.Usagetype != "" && record.Usagetype != "" {
		req.Header.Set(a.headers.Usagetype, record.Usagetype)
	}
}
