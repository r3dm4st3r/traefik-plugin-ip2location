# Troubleshooting Guide

## Common Issues and Solutions

### 1. Plugin Not Loading in Traefik

**Symptoms:**
- Traefik logs show: "Plugins are disabled because an error has occurred"
- Error mentions Yaegi interpreter or import errors

**Diagnosis:**
```bash
# Check Traefik logs
docker logs traefik 2>&1 | grep -i plugin
# or
journalctl -u traefik | grep -i plugin
```

**Common Causes:**
- External dependencies (should be none)
- Syntax errors in Go code
- Undefined variables or functions
- Missing imports

**Solution:**
- Verify `go.mod` has no dependencies
- Check all code compiles: `go build .`
- Review Traefik error logs for specific line numbers

---

### 2. Database File Not Found

**Symptoms:**
- Error: "failed to open MMDB file: no such file or directory"
- Headers not being set

**Diagnosis:**
```bash
# Check if file exists
ls -la /path/to/GeoLite2-City.mmdb

# Check file permissions
stat /path/to/GeoLite2-City.mmdb

# Verify it's a valid MMDB file
file /path/to/GeoLite2-City.mmdb
```

**Solution:**
- Use absolute path in configuration
- Ensure file is readable by Traefik process
- Verify file is actually a .mmdb file (not .bin)

---

### 3. IP Lookup Failing

**Symptoms:**
- Headers not populated
- X-GEOIP-ERROR header present
- Error: "database lookup failed" or "IP not found"

**Diagnosis:**
```bash
# Test the database directly
cd test_mmdb
go run main.go /path/to/GeoLite2-City.mmdb 8.8.8.8

# Check what IP Traefik sees
# Add logging to see detected IP
```

**Common Causes:**
- Wrong database type (ASN vs City)
- IP not in database
- Database corruption
- Metadata reading issues

**Solution:**
- Use GeoLite2-City.mmdb for full geo data
- Test with known IPs (8.8.8.8, 1.1.1.1)
- Re-download database file
- Check database file size (should be ~50-70MB for City)

---

### 4. Headers Not Appearing in Response

**Symptoms:**
- Request headers work but response headers don't
- Client doesn't see geo headers

**Diagnosis:**
```bash
# Test with curl
curl -v http://your-domain.com

# Check response headers
curl -I http://your-domain.com | grep -i geo
```

**Solution:**
- Verify `addResponseHeaders()` is being called
- Check middleware order in Traefik
- Ensure headers are set before `next.ServeHTTP()`
- Check if another middleware is stripping headers

---

### 5. Wrong IP Being Detected

**Symptoms:**
- Headers show wrong country/location
- Behind proxy/load balancer

**Diagnosis:**
```bash
# Check what IP Traefik sees
# Add temporary logging in getIP() method
```

**Solution:**
- Configure `trustedProxies` with your proxy IPs
- Set `fromHeader` if using custom header
- Verify X-Forwarded-For or X-Real-IP headers
- Test with direct connection (bypass proxy)

---

### 6. Performance Issues

**Symptoms:**
- Slow response times
- High CPU usage
- Database file I/O errors

**Diagnosis:**
```bash
# Monitor Traefik performance
docker stats traefik

# Check file I/O
iostat -x 1
```

**Solution:**
- Ensure database file is on fast storage (SSD)
- Consider caching frequently accessed IPs
- Use smaller database (Country vs City)
- Check file permissions and access

---

## Diagnostic Commands

### Test Database File
```bash
cd test_mmdb
go run main.go /path/to/GeoLite2-City.mmdb 8.8.8.8
```

### Check Traefik Configuration
```bash
# Validate static config
traefik validate --configFile=traefik.yml

# Check dynamic config
traefik validate --configFile=dynamic.yml
```

### Test Plugin Loading
```bash
# Build plugin locally first
go build .

# Check for compilation errors
go vet ./...
go test ./...
```

### Monitor Traefik Logs
```bash
# Docker
docker logs -f traefik

# Systemd
journalctl -u traefik -f

# Check for plugin errors
docker logs traefik 2>&1 | grep -i "plugin\|geoip\|error"
```

---

## Debug Mode

To enable more verbose logging, you can temporarily add debug statements:

1. **Add IP detection logging:**
```go
log.Printf("[geoip] Detected IP: %s from RemoteAddr: %s", ip.String(), req.RemoteAddr)
```

2. **Add lookup result logging:**
```go
log.Printf("[geoip] Lookup result for %s: Country=%s, City=%s", ip.String(), record.Country.IsoCode, record.City.Names["en"])
```

3. **Add header setting logging:**
```go
log.Printf("[geoip] Setting header %s = %s", headerName, value)
```

---

## Error Code Reference

| Error Message | Cause | Solution |
|--------------|-------|----------|
| `filename is required` | Missing database path | Add `filename` to config |
| `failed to open MMDB file` | File not found/not readable | Check path and permissions |
| `metadata marker not found` | Invalid/corrupted MMDB file | Re-download database |
| `could not determine client IP` | IP parsing failed | Check RemoteAddr format |
| `database lookup failed` | IP not in database or DB error | Test with known IP, check DB |
| `IP not found in database` | IP not in database range | Normal for some IPs |

---

## Getting Help

If issues persist:

1. **Collect Information:**
   - Traefik version
   - Plugin version
   - Full error logs
   - Configuration files (sanitized)
   - Database file type and size

2. **Test Components:**
   - Database file works with test script
   - Plugin compiles without errors
   - Traefik loads plugin successfully

3. **Check Logs:**
   - Traefik startup logs
   - Plugin loading errors
   - Runtime errors during requests

---

## Quick Health Check

Run these commands to verify everything is working:

```bash
# 1. Code compiles
go build .

# 2. Database file exists and is readable
test -r /path/to/GeoLite2-City.mmdb && echo "OK" || echo "FAIL"

# 3. Test database lookup
cd test_mmdb && go run main.go /path/to/GeoLite2-City.mmdb 8.8.8.8

# 4. Traefik can load plugin (check logs)
docker logs traefik | grep -i "plugin.*ip2location"

# 5. Test HTTP request
curl -v http://your-domain.com | grep -i "X-GEO"
```

All should return "OK" or show expected output.

