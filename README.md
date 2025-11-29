# IP2Location Traefik Plugin

Traefik middleware plugin for enriching requests with geolocation information from IP2Location BIN database (.bin format).

**Compatible with Traefik v3.6+**

## Features

- ✅ **Traefik v3.6 Compatible** - Fully updated for the latest Traefik version
- ✅ **IP2Location BIN Support** - Uses IP2Location BIN database format (.bin)
- ✅ **Enhanced IP Detection** - Supports X-Forwarded-For, X-Real-IP, and custom headers
- ✅ **Trusted Proxy Support** - Configure trusted proxy IP ranges for security
- ✅ **Comprehensive Geo Data** - Country, Region, City, Latitude, Longitude, ISP, Domain, and more
- ✅ **IPv4 and IPv6 Support** - Works with both IP address versions
- ✅ **Error Handling** - Configurable error reporting
- ✅ **High Performance** - Efficient IP2Location database lookups
- ✅ **No External Dependencies** - Works with Traefik's Yaegi interpreter

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
          filename: /path/to/IP2LOCATION-LITE-DB11.BIN
          fromHeader: "" # Optional: custom header to read IP from
          useXForwardedFor: true # Use X-Forwarded-For header (default: true)
          useXRealIP: true # Use X-Real-IP header (default: true)
          trustedProxies: # Optional: list of trusted proxy CIDR ranges
            - "10.0.0.0/8"
            - "172.16.0.0/12"
            - "192.168.0.0/16"
          disableErrorHeader: false # Set to true to disable error headers
          # Header mappings - flattened (no nested "headers:" for Yaegi compatibility)
          country_code: X-GEO-Country-Code
          country_name: X-GEO-Country-Name
          region: X-GEO-Region
          region_code: X-GEO-Region-Code
          city: X-GEO-City
          postal_code: X-GEO-Postal-Code
          latitude: X-GEO-Latitude
          longitude: X-GEO-Longitude
          timezone: X-GEO-TimeZone
          continent_code: X-GEO-Continent-Code
          continent_name: X-GEO-Continent-Name
          isp: X-GEO-ISP
          asn: X-GEO-ASN
          asn_organization: X-GEO-ASN-Org
          domain: X-GEO-Domain
          connection_type: X-GEO-Connection-Type
          user_type: X-GEO-User-Type
          accuracy_radius: X-GEO-Accuracy-Radius
```

### Minimal Configuration Example

```yaml
http:
  middlewares:
    geo-headers:
      plugin:
        ip2location:
          filename: /data/IP2LOCATION-LITE-DB11.BIN
          country_code: X-Country-Code
          city: X-City
```

### Legacy Field Names (Backward Compatibility)

For backward compatibility, the following legacy field names are supported:

```yaml
country_short: X-GEO-Country  # Maps to country_code
country_long: X-GEO-Country-Name  # Maps to country_name
zipcode: X-GEO-Zipcode  # Maps to postal_code
```

## Configuration Options

### Filename (`filename`)

**Required**

The absolute path to the IP2Location BIN database file (.bin format).

Supported database types:
- `IP2LOCATION-LITE-DB11.BIN` - City-level data with ISP (recommended)
- `IP2LOCATION-LITE-DB1.BIN` - Country-level data only
- `IP2LOCATION-LITE-DB3.BIN` - Region-level data
- `IP2LOCATION-LITE-DB5.BIN` - City-level data
- Commercial databases (DB1-DB25) with various data fields

Example: `/data/IP2LOCATION-LITE-DB11.BIN`

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

### Header Mappings (Flattened Configuration)

**Default: empty**

Header mappings are configured directly at the root level (flattened) for Traefik Yaegi compatibility. Only configured headers will be added to requests and responses.

**Note:** The configuration is flattened (no nested `headers:` section) because Traefik's Yaegi interpreter has limitations with nested struct parsing.

## Available Fields

### Location Fields

- `CountryCode` / `CountryShort` - ISO 3166-1 alpha-2 country code (e.g., "US")
- `CountryName` / `CountryLong` - Country name in English (e.g., "United States")
- `Region` - Subdivision (state/province) name in English
- `City` - City name in English
- `PostalCode` / `Zipcode` - Postal/ZIP code
- `Latitude` - Latitude coordinate (float32, 6 decimal precision)
- `Longitude` - Longitude coordinate (float32, 6 decimal precision)
- `Timezone` - Time zone (e.g., "America/New_York")

### Network Fields

- `Isp` - Internet Service Provider name
- `Domain` - Domain name associated with the IP

### Additional IP2Location Fields (depending on database type)

- `Netspeed` - Internet connection speed
- `Iddcode` - International Direct Dialing code
- `Areacode` - Area code
- `Weatherstationcode` - Weather station code
- `Weatherstationname` - Weather station name
- `Mcc` - Mobile country code
- `Mnc` - Mobile network code
- `Mobilebrand` - Mobile carrier brand
- `Elevation` - Elevation in meters
- `Usagetype` - Usage type

**Note:** IP2Location databases have different field availability depending on the database type (DB1-DB25). Higher-numbered databases include more fields.

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
- IP2Location BIN database file (.bin format)

## No External Dependencies

This plugin is designed to work with Traefik's plugin system, which does not support external Go module dependencies. The plugin includes a built-in IP2Location BIN reader implementation using only Go's standard library. This ensures compatibility with Traefik's plugin loading mechanism.

## Building from Source

```bash
go mod download
go build -o ip2location.so -buildmode=plugin .
```

## Migration from MaxMind

If you're migrating from MaxMind MMDB format:

1. **Download IP2Location Database**: Get `IP2LOCATION-LITE-DB11.BIN` from IP2Location
2. **Update Configuration**: Change `filename` to point to the .bin file
3. **Configure Headers**: Set up header mappings for the fields you need:
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
          filename: /etc/traefik/IP2LOCATION-LITE-DB11.BIN
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

- The plugin includes a built-in MMDB reader (no external dependencies)
- Database lookups are thread-safe and efficient
- The plugin maintains backward compatibility with legacy field names
- Error headers use `X-GEOIP-ERROR` (changed from `X-IP2LOCATION-ERROR`)
- Compatible with Traefik's plugin system (no external Go module dependencies required)
