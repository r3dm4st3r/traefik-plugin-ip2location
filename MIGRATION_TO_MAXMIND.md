# Migration Guide: IP2Location to MaxMind GeoIP2

This document explains the changes made when migrating from IP2Location BIN format to MaxMind GeoIP2 .mmdb format.

## Overview

The plugin has been completely rewritten to use MaxMind GeoIP2 databases instead of IP2Location. This provides:
- Better database availability (free GeoLite2 databases)
- More standardized format (.mmdb)
- Additional fields (ASN, Connection Type, User Type)
- Active development and updates

## Breaking Changes

### Database Format
- **Old**: IP2Location BIN format (`.BIN` files)
- **New**: MaxMind GeoIP2 MMDB format (`.mmdb` files)

### Error Header Name
- **Old**: `X-IP2LOCATION-ERROR`
- **New**: `X-GEOIP-ERROR`

### Field Name Changes

Most field names have been updated to match MaxMind's naming convention:

| Old (IP2Location) | New (MaxMind) | Notes |
|------------------|---------------|-------|
| `CountryShort` | `CountryCode` | Legacy name still supported |
| `CountryLong` | `CountryName` | Legacy name still supported |
| `Zipcode` | `PostalCode` | Legacy name still supported |
| `Netspeed` | ❌ Removed | Not available in MaxMind |
| `Iddcode` | ❌ Removed | Not available in MaxMind |
| `Areacode` | ❌ Removed | Not available in MaxMind |
| `Weatherstationcode` | ❌ Removed | Not available in MaxMind |
| `Weatherstationname` | ❌ Removed | Not available in MaxMind |
| `Mcc` | ❌ Removed | Not available in MaxMind |
| `Mnc` | ❌ Removed | Not available in MaxMind |
| `Mobilebrand` | ❌ Removed | Not available in MaxMind |
| `Elevation` | ❌ Removed | Not available in MaxMind |
| `Usagetype` | `UserType` | Similar concept, different name |

### New Fields Available

MaxMind provides additional fields not available in IP2Location:

- `RegionCode` - ISO subdivision code
- `ContinentCode` - Continent code (e.g., "NA")
- `ContinentName` - Continent name
- `Asn` - Autonomous System Number
- `AsnOrganization` - ASN organization name
- `ConnectionType` - Connection type (cable, dialup, etc.)
- `AccuracyRadius` - Accuracy radius in kilometers

## Migration Steps

### 1. Download MaxMind Database

Download a MaxMind GeoIP2 database:
- **Free**: [GeoLite2-City.mmdb](https://dev.maxmind.com/geoip/geoip2/geolite2/)
- **Commercial**: [GeoIP2-City.mmdb](https://www.maxmind.com/en/geoip2-databases)

### 2. Update Configuration

#### Before (IP2Location):
```yaml
http:
  middlewares:
    geo:
      plugin:
        ip2location:
          filename: /path/to/IP2LOCATION-LITE-DB1.IPV6.BIN
          headers:
            CountryShort: X-Country
            City: X-City
```

#### After (MaxMind):
```yaml
http:
  middlewares:
    geo:
      plugin:
        ip2location:
          filename: /path/to/GeoLite2-City.mmdb
          headers:
            CountryCode: X-Country
            City: X-City
```

### 3. Update Field Names

Update your configuration to use new field names:

```yaml
headers:
  # Old names (still work for backward compatibility)
  CountryShort: X-Country-Code
  CountryLong: X-Country-Name
  Zipcode: X-Zipcode
  
  # New names (recommended)
  CountryCode: X-Country-Code
  CountryName: X-Country-Name
  PostalCode: X-Postal-Code
  RegionCode: X-Region-Code
  ContinentCode: X-Continent-Code
  ContinentName: X-Continent-Name
  Asn: X-ASN
  AsnOrganization: X-ASN-Org
  ConnectionType: X-Connection-Type
  UserType: X-User-Type
  AccuracyRadius: X-Accuracy-Radius
```

### 4. Remove Unsupported Fields

Remove configuration for fields that are no longer available:
- `Netspeed`
- `Iddcode`
- `Areacode`
- `Weatherstationcode`
- `Weatherstationname`
- `Mcc`
- `Mnc`
- `Mobilebrand`
- `Elevation`

### 5. Update Error Handling

If your application checks for `X-IP2LOCATION-ERROR`, update it to check for `X-GEOIP-ERROR`:

```yaml
# Old
disableErrorHeader: false  # Errors in X-IP2LOCATION-ERROR

# New
disableErrorHeader: false  # Errors in X-GEOIP-ERROR
```

## Backward Compatibility

The plugin maintains backward compatibility for these field names:
- `CountryShort` → Maps to `CountryCode`
- `CountryLong` → Maps to `CountryName`
- `Zipcode` → Maps to `PostalCode`

However, it's recommended to update to the new field names for clarity.

## Database Comparison

| Feature | IP2Location | MaxMind GeoIP2 |
|---------|-------------|----------------|
| Free Database | ✅ Yes | ✅ Yes (GeoLite2) |
| Format | Binary (.BIN) | MMDB (.mmdb) |
| Update Frequency | Monthly | Weekly |
| IPv6 Support | ✅ Yes | ✅ Yes |
| ISP Data | ✅ Yes | ✅ Yes (commercial) |
| ASN Data | ❌ No | ✅ Yes |
| Mobile Data | ✅ Yes | ❌ No |
| Weather Data | ✅ Yes | ❌ No |
| File Size (City) | ~50-100 MB | ~50-70 MB |

## Testing

After migration, test your configuration:

1. **Verify Database Loads**: Check Traefik logs for database loading errors
2. **Test IP Lookups**: Verify headers are being set correctly
3. **Check Error Handling**: Test with invalid IPs to ensure error headers work
4. **Verify Proxy Headers**: Test X-Forwarded-For and X-Real-IP if used

## Troubleshooting

### Database Not Found
```
Error: error opening MaxMind database file: open /path/to/GeoLite2-City.mmdb: no such file or directory
```
**Solution**: Ensure the database file path is correct and the file exists.

### Invalid Database Format
```
Error: error opening MaxMind database file: invalid database format
```
**Solution**: Ensure you're using a MaxMind GeoIP2 .mmdb file, not an IP2Location .BIN file.

### Missing Fields
If certain fields are not being populated:
- Check that your database type supports those fields (e.g., City data requires GeoLite2-City, not GeoLite2-Country)
- Some fields (ISP, ASN) are only available in commercial GeoIP2 databases, not free GeoLite2

## Support

For issues or questions:
- Check the README.md for configuration examples
- Review test files for usage examples
- Ensure you're using a valid MaxMind GeoIP2 database file

## Additional Resources

- [MaxMind GeoIP2 Documentation](https://dev.maxmind.com/geoip/docs)
- [MaxMind GeoLite2 Download](https://dev.maxmind.com/geoip/geoip2/geolite2/)
- [MaxMind GeoIP2 Go Library](https://github.com/oschwald/geoip2-golang)

