package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/r3dm4st3r/traefik-plugin-ip2location"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path-to-geolite2-city.mmdb> [ip-address]")
		fmt.Println("Example: go run main.go ../GeoLite2-City.mmdb 8.8.8.8")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Opening MMDB file: %s\n", dbPath)

	db, err := traefik_plugin_ip2location.OpenMMDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open MMDB: %v", err)
	}
	defer db.Close()

	fmt.Println("✓ MMDB file opened successfully")

	// Test IP lookup if provided
	if len(os.Args) >= 3 {
		ipStr := os.Args[2]
		fmt.Printf("\nLooking up IP: %s\n", ipStr)

		ip := net.ParseIP(ipStr)
		if ip == nil {
			log.Fatalf("Invalid IP address: %s", ipStr)
		}

		record, err := db.LookupIP(ip)
		if err != nil {
			log.Fatalf("Failed to lookup IP: %v", err)
		}

		fmt.Println("\n✓ Lookup successful!")
		
		// ASN-specific fields (for GeoLite2-ASN database)
		if record.Traits.AutonomousSystemNumber != 0 {
			fmt.Printf("ASN: %d\n", record.Traits.AutonomousSystemNumber)
		}
		if record.Traits.AutonomousSystemOrganization != "" {
			fmt.Printf("ASN Organization: %s\n", record.Traits.AutonomousSystemOrganization)
		}
		
		// Geographic fields (for GeoLite2-City database)
		if record.Country.IsoCode != "" {
			fmt.Printf("Country Code: %s\n", record.Country.IsoCode)
		}
		if name, ok := record.Country.Names["en"]; ok && name != "" {
			fmt.Printf("Country Name: %s\n", name)
		}
		if len(record.Subdivisions) > 0 {
			if name, ok := record.Subdivisions[0].Names["en"]; ok && name != "" {
				fmt.Printf("Region: %s\n", name)
			}
			if record.Subdivisions[0].IsoCode != "" {
				fmt.Printf("Region Code: %s\n", record.Subdivisions[0].IsoCode)
			}
		}
		if name, ok := record.City.Names["en"]; ok && name != "" {
			fmt.Printf("City: %s\n", name)
		}
		if record.Postal.Code != "" {
			fmt.Printf("Postal Code: %s\n", record.Postal.Code)
		}
		if record.Location.Latitude != 0 || record.Location.Longitude != 0 {
			fmt.Printf("Latitude: %.6f\n", record.Location.Latitude)
			fmt.Printf("Longitude: %.6f\n", record.Location.Longitude)
		}
		if record.Location.TimeZone != "" {
			fmt.Printf("Timezone: %s\n", record.Location.TimeZone)
		}
		if record.Continent.Code != "" {
			fmt.Printf("Continent Code: %s\n", record.Continent.Code)
		}
		if name, ok := record.Continent.Names["en"]; ok && name != "" {
			fmt.Printf("Continent Name: %s\n", name)
		}
		if record.Traits.ISP != "" {
			fmt.Printf("ISP: %s\n", record.Traits.ISP)
		}
		if record.Traits.Domain != "" {
			fmt.Printf("Domain: %s\n", record.Traits.Domain)
		}
		if record.Traits.ConnectionType != "" {
			fmt.Printf("Connection Type: %s\n", record.Traits.ConnectionType)
		}
		if record.Traits.UserType != "" {
			fmt.Printf("User Type: %s\n", record.Traits.UserType)
		}
	} else {
		fmt.Println("\n✓ MMDB file structure is valid")
		fmt.Println("To test IP lookup, provide an IP address as second argument")
		fmt.Println("\nExample test IPs:")
		fmt.Println("  - 8.8.8.8 (Google DNS)")
		fmt.Println("  - 1.1.1.1 (Cloudflare DNS)")
		fmt.Println("  - 208.67.222.222 (OpenDNS)")
	}
}

