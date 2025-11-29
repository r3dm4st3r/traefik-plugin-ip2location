#!/bin/bash
# Diagnostic script for Traefik IP2Location Plugin

echo "=== Traefik IP2Location Plugin Diagnostic Tool ==="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check 1: Go installation
echo "1. Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓${NC} Go installed: $GO_VERSION"
else
    echo -e "${RED}✗${NC} Go not found"
fi
echo ""

# Check 2: Code compilation
echo "2. Checking code compilation..."
if go build . &> /dev/null; then
    echo -e "${GREEN}✓${NC} Code compiles successfully"
else
    echo -e "${RED}✗${NC} Compilation errors:"
    go build . 2>&1 | head -10
fi
echo ""

# Check 3: Go vet
echo "3. Running go vet..."
if go vet ./... &> /dev/null; then
    echo -e "${GREEN}✓${NC} No vet issues"
else
    echo -e "${YELLOW}⚠${NC} Vet warnings:"
    go vet ./... 2>&1
fi
echo ""

# Check 4: Dependencies
echo "4. Checking dependencies..."
if [ -f go.mod ]; then
    DEPS=$(grep -v "^module\|^go " go.mod | grep -v "^$" | wc -l)
    if [ "$DEPS" -eq 0 ]; then
        echo -e "${GREEN}✓${NC} No external dependencies (required for Traefik plugins)"
    else
        echo -e "${RED}✗${NC} Found $DEPS external dependencies (plugins can't use external deps)"
        grep -v "^module\|^go " go.mod | grep -v "^$"
    fi
else
    echo -e "${YELLOW}⚠${NC} go.mod not found"
fi
echo ""

# Check 5: Test database file (if provided)
if [ -n "$1" ]; then
    echo "5. Testing database file: $1"
    if [ -f "$1" ]; then
        FILE_SIZE=$(stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null)
        echo -e "${GREEN}✓${NC} File exists, size: $FILE_SIZE bytes"
        
        # Check if it's a BIN file
        if [[ "$1" == *.bin ]] || [[ "$1" == *.BIN ]]; then
            echo -e "${GREEN}✓${NC} File has .bin extension (IP2Location format)"
        else
            echo -e "${YELLOW}⚠${NC} File doesn't have .bin extension: $(file "$1")"
        fi
        
        # Check file type
        FILE_TYPE=$(file "$1" 2>/dev/null)
        echo "   File type: $FILE_TYPE"
        
        # Test lookup if test script exists
        if [ -d "test_mmdb" ] && [ -f "test_mmdb/main.go" ]; then
            echo "   Testing lookup with 8.8.8.8..."
            cd test_mmdb
            if go run main.go "../$1" 8.8.8.8 &> /dev/null; then
                echo -e "${GREEN}✓${NC} Database lookup test passed"
                go run main.go "../$1" 8.8.8.8
            else
                echo -e "${RED}✗${NC} Database lookup test failed"
                go run main.go "../$1" 8.8.8.8
            fi
            cd ..
        fi
    else
        echo -e "${RED}✗${NC} Database file not found: $1"
    fi
else
    echo "5. Database file test (provide path as argument)"
    echo "   Usage: ./diagnose.sh /path/to/IP2LOCATION-LITE-DB11.BIN"
fi
echo ""

# Check 6: Traefik plugin files
echo "6. Checking plugin files..."
if [ -f ".traefik.yml" ]; then
    echo -e "${GREEN}✓${NC} .traefik.yml found"
else
    echo -e "${YELLOW}⚠${NC} .traefik.yml not found"
fi

if [ -f "ip2location.go" ]; then
    echo -e "${GREEN}✓${NC} ip2location.go found"
else
    echo -e "${RED}✗${NC} ip2location.go not found"
fi

if [ -f "lib.go" ]; then
    echo -e "${GREEN}✓${NC} lib.go found (IP2Location BIN reader)"
else
    echo -e "${RED}✗${NC} lib.go not found (required for IP2Location BIN support)"
fi

# Check if old MMDB file still exists (should be removed)
if [ -f "mmdb.go" ]; then
    echo -e "${YELLOW}⚠${NC} mmdb.go still exists (should be removed - using IP2Location now)"
fi
echo ""

# Check 7: Common issues
echo "7. Checking for common issues..."

# Check for external imports
if grep -r "github.com/oschwald\|github.com/ip2location" *.go 2>/dev/null | grep -v "^lib.go.*// https://github.com/ip2location"; then
    echo -e "${RED}✗${NC} Found external dependency imports"
    grep -r "github.com/oschwald\|github.com/ip2location" *.go 2>/dev/null | grep -v "^lib.go.*// https://github.com/ip2location"
else
    echo -e "${GREEN}✓${NC} No external dependency imports"
fi

# Check for MMDB references (should be removed)
if grep -r "MMDB\|MaxMind\|\.mmdb" *.go 2>/dev/null | grep -v "^lib.go.*//"; then
    echo -e "${YELLOW}⚠${NC} Found MMDB/MaxMind references (should be removed)"
    grep -r "MMDB\|MaxMind\|\.mmdb" *.go 2>/dev/null | grep -v "^lib.go.*//"
else
    echo -e "${GREEN}✓${NC} No MMDB/MaxMind references found"
fi

# Check for undefined variables
if go build . 2>&1 | grep -q "undefined"; then
    echo -e "${RED}✗${NC} Undefined variables/functions found"
    go build . 2>&1 | grep "undefined"
else
    echo -e "${GREEN}✓${NC} No undefined variables"
fi
echo ""

echo "=== Diagnostic Complete ==="
echo ""
echo "Next steps:"
echo "1. Review any errors above"
echo "2. Check TROUBLESHOOTING.md for solutions"
echo "3. Ensure you have an IP2Location BIN database file (.bin)"
echo "4. Check Traefik logs for plugin errors"
echo ""
echo "Database file requirements:"
echo "  - Must be IP2Location BIN format (.bin extension)"
echo "  - Recommended: IP2LOCATION-LITE-DB11.BIN (City + ISP)"
echo "  - Download from: https://lite.ip2location.com/"

