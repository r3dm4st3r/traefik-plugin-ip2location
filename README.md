# ip2location

Traefik middleware plugin for enriching requests with geolocation information from IP2Location database.

**Compatible with Traefik v3.6+**

## Features

- ✅ **Traefik v3.6 Compatible** - Fully updated for the latest Traefik version
- ✅ **Enhanced IP Detection** - Supports X-Forwarded-For, X-Real-IP, and custom headers
- ✅ **Trusted Proxy Support** - Configure trusted proxy IP ranges for security
- ✅ **Comprehensive Geo Data** - 20+ geolocation fields available
- ✅ **IPv4 and IPv6 Support** - Works with both IP address versions
- ✅ **Error Handling** - Configurable error reporting
- ✅ **High Performance** - Efficient binary database lookups

## Configuration

### Static Configuration (Traefik v3)

For Traefik v3, plugins are configured using the static configuration. Here's an example:

```yaml
# Static configuration
experimental:
  plugins:
    ip2location:
      moduleName: github.com/r3dm4st3r/traefik-plugin-ip2location
      version: v1.0.1
```

### Dynamic Configuration

Add the middleware configuration to your Traefik dynamic configuration:

```yaml
http:
  middlewares:
    ip2location-geo:
      plugin:
        ip2location:
          filename: /path/to/IP2LOCATION-LITE-DB1.IPV6.BIN
          fromHeader: "" # Optional: custom header to read IP from
          useXForwardedFor: true # Use X-Forwarded-For header (default: true)
          useXRealIP: true # Use X-Real-IP header (default: true)
          trustedProxies: # Optional: list of trusted proxy CIDR ranges
            - "10.0.0.0/8"
            - "172.16.0.0/12"
            - "192.168.0.0/16"
          disableErrorHeader: false # Set to true to disable error headers
          headers:
            CountryShort: X-GEO-CountryShort
            CountryLong: X-GEO-CountryLong
            Region: X-GEO-Region
            City: X-GEO-City
            Isp: X-GEO-Isp
            Latitude: X-GEO-Latitude
            Longitude: X-GEO-Longitude
            Domain: X-GEO-Domain
            Zipcode: X-GEO-Zipcode
            Timezone: X-GEO-Timezone
            Netspeed: X-GEO-Netspeed
            Iddcode: X-GEO-Iddcode
            Areacode: X-GEO-Areacode
            Weatherstationcode: X-GEO-Weatherstationcode
            Weatherstationname: X-GEO-Weatherstationname
            Mcc: X-GEO-Mcc
            Mnc: X-GEO-Mnc
            Mobilebrand: X-GEO-Mobilebrand
            Elevation: X-GEO-Elevation
            Usagetype: X-GEO-Usagetype
```

### Minimal Configuration Example

```yaml
http:
  middlewares:
    geo-headers:
      plugin:
        ip2location:
          filename: /data/IP2LOCATION-LITE-DB1.IPV6.BIN
          headers:
            CountryShort: X-Country-Code
            City: X-City
```

## Configuration Options

### Filename (`filename`)

**Required**

The absolute path to the IP2Location database file (BIN format).

Example: `/data/IP2LOCATION-LITE-DB1.IPV6.BIN`

### FromHeader (`fromHeader`)

**Default: empty**

If specified, the IP address will be read from this HTTP header instead of using the default detection logic. This takes the highest priority.

Example: `X-User-IP`

### UseXForwardedFor (`useXForwardedFor`)

**Default: `true`**

Enable reading the client IP from the `X-Forwarded-For` header. Only used if the request comes from a trusted proxy (see `trustedProxies`).

### UseXRealIP (`useXRealIP`)

**Default: `true`**

Enable reading the client IP from the `X-Real-IP` header. Only used if the request comes from a trusted proxy (see `trustedProxies`).

### TrustedProxies (`trustedProxies`)

**Default: empty (all proxies trusted)**

List of CIDR ranges for trusted proxies. If empty, all proxies are trusted (backward compatible behavior).

**Security Note**: In production, it's recommended to configure this to only trust your load balancer/proxy IPs.

Examples:
```yaml
trustedProxies:
  - "10.0.0.0/8"        # Private network
  - "172.16.0.0/12"     # Private network
  - "192.168.0.0/16"    # Private network
  - "203.0.113.0/24"    # Specific proxy range
```

### DisableErrorHeader (`disableErrorHeader`)

**Default: `false`**

If `false`, errors will be added to the `X-IP2LOCATION-ERROR` HTTP header. Set to `true` to disable error headers.

### Headers (`headers`)

**Default: empty**

Map of IP2Location fields to HTTP header names. Only configured headers will be added to requests.

Available fields:
- `CountryShort` - ISO-3166 country code (e.g., "US")
- `CountryLong` - Country name (e.g., "United States")
- `Region` - Region/State name
- `City` - City name
- `Isp` - Internet Service Provider
- `Latitude` - Latitude coordinate (6 decimal precision)
- `Longitude` - Longitude coordinate (6 decimal precision)
- `Domain` - Domain name
- `Zipcode` - Postal/ZIP code
- `Timezone` - Time zone (e.g., "America/New_York")
- `Netspeed` - Connection speed category
- `Iddcode` - International Direct Dialing code
- `Areacode` - Area code
- `Weatherstationcode` - Weather station code
- `Weatherstationname` - Weather station name
- `Mcc` - Mobile Country Code
- `Mnc` - Mobile Network Code
- `Mobilebrand` - Mobile carrier brand
- `Elevation` - Elevation in meters (2 decimal precision)
- `Usagetype` - Usage type (e.g., "ISP", "DCH", "CDN")

## IP Detection Priority

The plugin detects the client IP address in the following order:

1. **Custom Header** (if `fromHeader` is configured)
2. **X-Real-IP** (if `useXRealIP` is `true` and proxy is trusted)
3. **X-Forwarded-For** (if `useXForwardedFor` is `true` and proxy is trusted) - takes first IP from comma-separated list
4. **RemoteAddr** - Direct connection IP

## Error Handling

If any error occurs during IP detection or database lookup, the error message will be added to the `X-IP2LOCATION-ERROR` header (unless `disableErrorHeader` is `true`). The request will continue to be processed normally.

## Database Files

Download IP2Location database files from:
- [IP2Location LITE (Free)](https://lite.ip2location.com/)
- [IP2Location Commercial](https://www.ip2location.com/)

Supported database types:
- DB1 through DB25 (all fields)
- IPv4 and IPv6 databases

## Requirements

- Traefik v3.0 or higher
- Go 1.21 or higher (for building from source)
- IP2Location BIN database file

## Building from Source

```bash
go mod download
go build -o ip2location.so -buildmode=plugin .
```

## License

See LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.