# MaxMind GeoIP2 Traefik Plugin

Traefik middleware plugin for enriching requests with geolocation information from MaxMind GeoIP2 database (.mmdb format).

**Compatible with Traefik v3.6+**

## Features

- ✅ **Traefik v3.6 Compatible** - Fully updated for the latest Traefik version
- ✅ **MaxMind GeoIP2 Support** - Uses MaxMind GeoIP2 database format (.mmdb)
- ✅ **Enhanced IP Detection** - Supports X-Forwarded-For, X-Real-IP, and custom headers
- ✅ **Trusted Proxy Support** - Configure trusted proxy IP ranges for security
- ✅ **Comprehensive Geo Data** - 20+ geolocation fields available
- ✅ **IPv4 and IPv6 Support** - Works with both IP address versions
- ✅ **Error Handling** - Configurable error reporting
- ✅ **High Performance** - Efficient MaxMind database lookups
- ✅ **Backward Compatible** - Legacy field names supported

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
    maxmind-geo:
      plugin:
        ip2location:
          filename: /path/to/GeoLite2-City.mmdb
          fromHeader: "" # Optional: custom header to read IP from
          useXForwardedFor: true # Use X-Forwarded-For header (default: true)
          useXRealIP: true # Use X-Real-IP header (default: true)
          trustedProxies: # Optional: list of trusted proxy CIDR ranges
            - "10.0.0.0/8"
            - "172.16.0.0/12"
            - "192.168.0.0/16"
          disableErrorHeader: false # Set to true to disable error headers
          headers:
            CountryCode: X-GEO-Country-Code
            CountryName: X-GEO-Country-Name
            Region: X-GEO-Region
            RegionCode: X-GEO-Region-Code
            City: X-GEO-City
            PostalCode: X-GEO-Postal-Code
            Latitude: X-GEO-Latitude
            Longitude: X-GEO-Longitude
            Timezone: X-GEO-TimeZone
            ContinentCode: X-GEO-Continent-Code
            ContinentName: X-GEO-Continent-Name
            Isp: X-GEO-ISP
            Asn: X-GEO-ASN
            AsnOrganization: X-GEO-ASN-Org
            Domain: X-GEO-Domain
            ConnectionType: X-GEO-Connection-Type
            UserType: X-GEO-User-Type
            AccuracyRadius: X-GEO-Accuracy-Radius
```

### Minimal Configuration Example

```yaml
http:
  middlewares:
    geo-headers:
      plugin:
        ip2location:
          filename: /data/GeoLite2-City.mmdb
          headers:
            CountryCode: X-Country-Code
            City: X-City
```

### Legacy Field Names (Backward Compatibility)

For backward compatibility, the following legacy field names are supported:

```yaml
headers:
  CountryShort: X-GEO-Country  # Maps to CountryCode
  CountryLong: X-GEO-Country-Name  # Maps to CountryName
  Zipcode: X-GEO-Zipcode  # Maps to PostalCode
```

## Configuration Options

### Filename (`filename`)

**Required**

The absolute path to the MaxMind GeoIP2 database file (.mmdb format).

Supported database types:
- `GeoLite2-City.mmdb` - City-level data (recommended)
- `GeoLite2-Country.mmdb` - Country-level data only
- `GeoIP2-City.mmdb` - Commercial City database
- `GeoIP2-Country.mmdb` - Commercial Country database

Example: `/data/GeoLite2-City.mmdb`

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

If `false`, errors will be added to the `X-GEOIP-ERROR` HTTP header. Set to `true` to disable error headers.

### Headers (`headers`)

**Default: empty**

Map of MaxMind GeoIP2 fields to HTTP header names. Only configured headers will be added to requests.

## Available Fields

### Location Fields

- `CountryCode` - ISO 3166-1 alpha-2 country code (e.g., "US")
- `CountryName` - Country name in English (e.g., "United States")
- `Region` - Subdivision (state/province) name in English
- `RegionCode` - ISO 3166-2 subdivision code
- `City` - City name in English
- `PostalCode` - Postal/ZIP code
- `Latitude` - Latitude coordinate (6 decimal precision)
- `Longitude` - Longitude coordinate (6 decimal precision)
- `Timezone` - Time zone (e.g., "America/New_York")
- `AccuracyRadius` - Accuracy radius in kilometers

### Continent Fields

- `ContinentCode` - Continent code (e.g., "NA")
- `ContinentName` - Continent name in English (e.g., "North America")

### Network Fields

- `Isp` - Internet Service Provider name
- `Asn` - Autonomous System Number
- `AsnOrganization` - Autonomous System Organization name
- `Domain` - Domain name associated with the IP
- `ConnectionType` - Connection type (e.g., "cable", "dialup")
- `UserType` - User type (e.g., "business", "residential")

### Legacy Fields (Backward Compatibility)

- `CountryShort` - Maps to `CountryCode`
- `CountryLong` - Maps to `CountryName`
- `Zipcode` - Maps to `PostalCode`

## IP Detection Priority

The plugin detects the client IP address in the following order:

1. **Custom Header** (if `fromHeader` is configured)
2. **X-Real-IP** (if `useXRealIP` is `true` and proxy is trusted)
3. **X-Forwarded-For** (if `useXForwardedFor` is `true` and proxy is trusted) - takes first IP from comma-separated list
4. **RemoteAddr** - Direct connection IP

## Error Handling

If any error occurs during IP detection or database lookup, the error message will be added to the `X-GEOIP-ERROR` header (unless `disableErrorHeader` is `true`). The request will continue to be processed normally.

## Database Files

### Free Databases (GeoLite2)

Download MaxMind GeoLite2 database files from:
- [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geoip2/geolite2/)
- Requires free MaxMind account registration

### Commercial Databases (GeoIP2)

For commercial databases with additional features:
- [MaxMind GeoIP2](https://www.maxmind.com/en/geoip2-databases)

### Database Types

- **GeoLite2-City** - City-level geolocation (recommended for most use cases)
- **GeoLite2-Country** - Country-level only (smaller file size)
- **GeoIP2-City** - Commercial city database with ISP/ASN data
- **GeoIP2-Country** - Commercial country database

### Updating Databases

MaxMind databases are updated regularly. You can:
1. Download updated databases manually
2. Use MaxMind's GeoIP Update tool: https://github.com/maxmind/geoipupdate
3. Set up automated updates via cron/systemd timer

## Requirements

- Traefik v3.0 or higher
- Go 1.21 or higher (for building from source)
- MaxMind GeoIP2 database file (.mmdb format)

## Building from Source

```bash
go mod download
go build -o ip2location.so -buildmode=plugin .
```

## Migration from IP2Location

If you're migrating from IP2Location format:

1. **Download MaxMind Database**: Get `GeoLite2-City.mmdb` from MaxMind
2. **Update Configuration**: Change `filename` to point to the .mmdb file
3. **Update Field Names**: Replace IP2Location-specific fields with MaxMind equivalents:
   - `CountryShort` → `CountryCode`
   - `CountryLong` → `CountryName`
   - `Zipcode` → `PostalCode`
4. **Test**: Verify headers are being set correctly

## Example: Complete Configuration

```yaml
# Static config
experimental:
  plugins:
    ip2location:
      moduleName: github.com/r3dm4st3r/traefik-plugin-ip2location
      version: v1.0.1

# Dynamic config
http:
  routers:
    myapp:
      rule: "Host(`example.com`)"
      middlewares:
        - geo-headers
      service: myapp

  middlewares:
    geo-headers:
      plugin:
        ip2location:
          filename: /etc/traefik/GeoLite2-City.mmdb
          trustedProxies:
            - "10.0.0.0/8"
          headers:
            CountryCode: X-Country-Code
            CountryName: X-Country-Name
            City: X-City
            Latitude: X-Latitude
            Longitude: X-Longitude
            Timezone: X-TimeZone
```

## License

See LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Notes

- The plugin uses MaxMind's official GeoIP2 Go library
- Database lookups are thread-safe and efficient
- The plugin maintains backward compatibility with legacy field names
- Error headers use `X-GEOIP-ERROR` (changed from `X-IP2LOCATION-ERROR`)
