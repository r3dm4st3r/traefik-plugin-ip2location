// Package traefik_plugin_ip2location implements a MaxMind GeoIP2 plugin for Traefik.
// This implementation includes a minimal MMDB reader to avoid external dependencies,
// as Traefik's plugin system does not support external Go module dependencies.
package traefik_plugin_ip2location

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"os"
)

// MMDBReader reads MaxMind DB (MMDB) format files
type MMDBReader struct {
	file     *os.File
	metadata *MMDBMetadata
	dataSection []byte
}

// MMDBMetadata contains MMDB file metadata
type MMDBMetadata struct {
	NodeCount     uint32
	RecordSize    uint16
	IPVersion     uint16
	DatabaseType string
	Languages     []string
	BinaryFormatMajorVersion uint16
	BinaryFormatMinorVersion uint16
	BuildEpoch    uint64
	Description   map[string]string
	DataSectionStart uint32
}

// GeoIP2Record contains parsed GeoIP2 data
type GeoIP2Record struct {
	Country struct {
		IsoCode string
		Names   map[string]string
	}
	Continent struct {
		Code  string
		Names map[string]string
	}
	City struct {
		Names map[string]string
	}
	Subdivisions []struct {
		IsoCode string
		Names   map[string]string
	}
	Postal struct {
		Code string
	}
	Location struct {
		Latitude      float64
		Longitude     float64
		TimeZone      string
		AccuracyRadius uint16
	}
	Traits struct {
		ISP                        string
		AutonomousSystemNumber     uint
		AutonomousSystemOrganization string
		Domain                     string
		ConnectionType             string
		UserType                   string
	}
}

// OpenMMDB opens a MaxMind DB file
func OpenMMDB(filename string) (*MMDBReader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open MMDB file: %w", err)
	}

	reader := &MMDBReader{
		file: file,
	}

	// Read metadata
	metadata, dataStart, err := reader.readMetadata()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	reader.metadata = metadata

	// Read data section into memory for faster access
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	dataSize := int64(dataStart) - 128 // Approximate, metadata is at end
	if dataSize < 0 {
		dataSize = fileInfo.Size() - int64(dataStart)
	}

	reader.dataSection = make([]byte, dataSize)
	file.Seek(0, 0)
	if _, err := file.Read(reader.dataSection); err != nil && err != io.EOF {
		file.Close()
		return nil, fmt.Errorf("failed to read data section: %w", err)
	}

	return reader, nil
}

// Close closes the MMDB file
func (r *MMDBReader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// LookupIP looks up an IP address in the MMDB database
func (r *MMDBReader) LookupIP(ip net.IP) (*GeoIP2Record, error) {
	if r.metadata == nil {
		return nil, fmt.Errorf("metadata not loaded")
	}

	// Convert IP to big integer for lookup
	var ipNum *big.Int
	var ipBits int

	if ip.To4() != nil {
		// IPv4
		ipNum = big.NewInt(0)
		ipNum.SetBytes(ip.To4())
		ipBits = 32
	} else {
		// IPv6
		ipNum = big.NewInt(0)
		ipNum.SetBytes(ip.To16())
		ipBits = 128
	}

	// Search the tree
	nodeNum := uint32(0)
	for i := 0; i < ipBits && nodeNum < r.metadata.NodeCount; i++ {
		bit := ipNum.Bit(ipBits - 1 - i)
		nodeNum = r.readNode(nodeNum, bit == 1)
	}

	if nodeNum >= r.metadata.NodeCount {
		// Data node
		dataOffset := nodeNum - r.metadata.NodeCount + r.metadata.DataSectionStart
		record, err := r.readDataRecord(dataOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to read data record: %w", err)
		}
		return record, nil
	}

	return nil, fmt.Errorf("IP not found in database")
}

// readNode reads a node from the tree
func (r *MMDBReader) readNode(nodeNum uint32, right bool) uint32 {
	recordSize := r.metadata.RecordSize
	offset := nodeNum * uint32(recordSize) * 2

	var node uint32
	if recordSize == 24 {
		// 24-bit records
		if right {
			offset += 3
		}
		buf := make([]byte, 3)
		r.file.ReadAt(buf, int64(offset))
		node = uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
	} else if recordSize == 28 {
		// 28-bit records
		buf := make([]byte, 4)
		r.file.ReadAt(buf, int64(offset))
		if right {
			node = (uint32(buf[3])&0x0F)<<24 | uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
		} else {
			node = uint32(buf[0])<<20 | uint32(buf[1])<<12 | uint32(buf[2])<<4 | uint32(buf[3])>>4
		}
	} else if recordSize == 32 {
		// 32-bit records
		if right {
			offset += 4
		}
		buf := make([]byte, 4)
		r.file.ReadAt(buf, int64(offset))
		node = binary.BigEndian.Uint32(buf)
	}

	return node
}

// readDataRecord reads a data record from the database
func (r *MMDBReader) readDataRecord(offset uint32) (*GeoIP2Record, error) {
	r.file.Seek(int64(offset), 0)
	
	record := &GeoIP2Record{
		Country: struct {
			IsoCode string
			Names   map[string]string
		}{Names: make(map[string]string)},
		Continent: struct {
			Code  string
			Names map[string]string
		}{Names: make(map[string]string)},
		City: struct {
			Names map[string]string
		}{Names: make(map[string]string)},
		Subdivisions: make([]struct {
			IsoCode string
			Names   map[string]string
		}, 0),
		Location: struct {
			Latitude      float64
			Longitude     float64
			TimeZone      string
			AccuracyRadius uint16
		}{},
		Traits: struct {
			ISP                        string
			AutonomousSystemNumber     uint
			AutonomousSystemOrganization string
			Domain                     string
			ConnectionType             string
			UserType                   string
		}{},
	}

	data, err := r.readData(offset)
	if err != nil {
		return nil, err
	}

	r.parseGeoIP2Data(data, record)
	return record, nil
}

// readData reads data from the specified offset
func (r *MMDBReader) readData(offset uint32) (map[string]interface{}, error) {
	r.file.Seek(int64(offset), 0)
	return r.decodeData()
}

// decodeData decodes MMDB data structure (must return a map)
func (r *MMDBReader) decodeData() (map[string]interface{}, error) {
	value, err := r.decodeValue()
	if err != nil {
		return nil, err
	}
	
	// Ensure we return a map
	if result, ok := value.(map[string]interface{}); ok {
		return result, nil
	}
	
	// If not a map, return empty map
	return make(map[string]interface{}), nil
}

// decodeValue decodes a value from MMDB
func (r *MMDBReader) decodeValue() (interface{}, error) {
	ctrlByte := make([]byte, 1)
	if _, err := r.file.Read(ctrlByte); err != nil {
		return nil, err
	}

	ctrl := ctrlByte[0]
	typeNum := ctrl >> 5

	if typeNum == 0 {
		// Extended type
		typeByte := make([]byte, 1)
		if _, err := r.file.Read(typeByte); err != nil {
			return nil, err
		}
		typeNum = typeByte[0] + 7
	}

	dataSize := int(ctrl & 0x1F)
	if dataSize >= 29 {
		bytesToRead := dataSize - 28
		sizeBytes := make([]byte, bytesToRead)
		if _, err := r.file.Read(sizeBytes); err != nil {
			return nil, err
		}
		dataSize = int(sizeBytes[0])
		if bytesToRead > 1 {
			for i := 1; i < bytesToRead; i++ {
				dataSize = dataSize<<8 | int(sizeBytes[i])
			}
		}
		dataSize += 28
	}

	switch typeNum {
	case 2: // UTF-8 string
		str := make([]byte, dataSize)
		if _, err := r.file.Read(str); err != nil {
			return nil, err
		}
		return string(str), nil
	case 3: // Double
		if dataSize != 8 {
			return nil, fmt.Errorf("invalid double size: %d", dataSize)
		}
		buf := make([]byte, 8)
		if _, err := r.file.Read(buf); err != nil {
			return nil, err
		}
		bits := binary.BigEndian.Uint64(buf)
		return float64FromBits(bits), nil
	case 4: // Bytes
		bytes := make([]byte, dataSize)
		if _, err := r.file.Read(bytes); err != nil {
			return nil, err
		}
		return bytes, nil
	case 5: // Unsigned 16-bit integer
		if dataSize != 2 {
			return nil, fmt.Errorf("invalid uint16 size: %d", dataSize)
		}
		buf := make([]byte, 2)
		if _, err := r.file.Read(buf); err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint16(buf), nil
	case 6: // Unsigned 32-bit integer
		if dataSize != 4 {
			return nil, fmt.Errorf("invalid uint32 size: %d", dataSize)
		}
		buf := make([]byte, 4)
		if _, err := r.file.Read(buf); err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint32(buf), nil
	case 7: // Map
		return r.decodeMap(dataSize)
	case 8: // Array
		return r.decodeArray(dataSize)
	case 14: // Boolean
		return dataSize != 0, nil
	default:
		// Skip unknown types
		skip := make([]byte, dataSize)
		r.file.Read(skip)
		return nil, nil
	}
}

// decodeMap decodes an MMDB map
func (r *MMDBReader) decodeMap(size int) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for i := 0; i < size; i++ {
		key, err := r.decodeString()
		if err != nil {
			return nil, err
		}
		
		value, err := r.decodeValue()
		if err != nil {
			return nil, err
		}
		
		result[key] = value
	}
	
	return result, nil
}

// decodeString decodes an MMDB string
func (r *MMDBReader) decodeString() (string, error) {
	ctrlByte := make([]byte, 1)
	if _, err := r.file.Read(ctrlByte); err != nil {
		return "", err
	}
	
	ctrl := ctrlByte[0]
	typeNum := ctrl >> 5
	dataSize := int(ctrl & 0x1F)
	
	if typeNum == 0 {
		typeByte := make([]byte, 1)
		if _, err := r.file.Read(typeByte); err != nil {
			return "", err
		}
		typeNum = typeByte[0] + 7
	}
	
	if dataSize >= 29 {
		bytesToRead := dataSize - 28
		sizeBytes := make([]byte, bytesToRead)
		if _, err := r.file.Read(sizeBytes); err != nil {
			return "", err
		}
		dataSize = int(sizeBytes[0])
		if bytesToRead > 1 {
			for i := 1; i < bytesToRead; i++ {
				dataSize = dataSize<<8 | int(sizeBytes[i])
			}
		}
		dataSize += 28
	}
	
	if typeNum != 2 {
		return "", fmt.Errorf("expected string type, got %d", typeNum)
	}
	
	str := make([]byte, dataSize)
	if _, err := r.file.Read(str); err != nil {
		return "", err
	}
	
	return string(str), nil
}

// decodeArray decodes an MMDB array
func (r *MMDBReader) decodeArray(size int) ([]interface{}, error) {
	result := make([]interface{}, size)
	
	for i := 0; i < size; i++ {
		value, err := r.decodeValue()
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	
	return result, nil
}

// float64FromBits converts uint64 bits to float64 using IEEE 754
func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// parseGeoIP2Data parses GeoIP2 data structure
func (r *MMDBReader) parseGeoIP2Data(data map[string]interface{}, record *GeoIP2Record) {
	if country, ok := data["country"].(map[string]interface{}); ok {
		if isoCode, ok := country["iso_code"].(string); ok {
			record.Country.IsoCode = isoCode
		}
		if names, ok := country["names"].(map[string]interface{}); ok {
			for lang, name := range names {
				if nameStr, ok := name.(string); ok {
					record.Country.Names[lang] = nameStr
				}
			}
		}
	}

	if continent, ok := data["continent"].(map[string]interface{}); ok {
		if code, ok := continent["code"].(string); ok {
			record.Continent.Code = code
		}
		if names, ok := continent["names"].(map[string]interface{}); ok {
			for lang, name := range names {
				if nameStr, ok := name.(string); ok {
					record.Continent.Names[lang] = nameStr
				}
			}
		}
	}

	if city, ok := data["city"].(map[string]interface{}); ok {
		if names, ok := city["names"].(map[string]interface{}); ok {
			for lang, name := range names {
				if nameStr, ok := name.(string); ok {
					record.City.Names[lang] = nameStr
				}
			}
		}
	}

	if subdivisions, ok := data["subdivisions"].([]interface{}); ok && len(subdivisions) > 0 {
		for _, sub := range subdivisions {
			if subMap, ok := sub.(map[string]interface{}); ok {
				subdiv := struct {
					IsoCode string
					Names   map[string]string
				}{Names: make(map[string]string)}
				
				if isoCode, ok := subMap["iso_code"].(string); ok {
					subdiv.IsoCode = isoCode
				}
				if names, ok := subMap["names"].(map[string]interface{}); ok {
					for lang, name := range names {
						if nameStr, ok := name.(string); ok {
							subdiv.Names[lang] = nameStr
						}
					}
				}
				record.Subdivisions = append(record.Subdivisions, subdiv)
			}
		}
	}

	if postal, ok := data["postal"].(map[string]interface{}); ok {
		if code, ok := postal["code"].(string); ok {
			record.Postal.Code = code
		}
	}

	if location, ok := data["location"].(map[string]interface{}); ok {
		if lat, ok := location["latitude"].(float64); ok {
			record.Location.Latitude = lat
		}
		if lon, ok := location["longitude"].(float64); ok {
			record.Location.Longitude = lon
		}
		if tz, ok := location["time_zone"].(string); ok {
			record.Location.TimeZone = tz
		}
		if acc, ok := location["accuracy_radius"].(uint16); ok {
			record.Location.AccuracyRadius = acc
		}
	}

	if traits, ok := data["traits"].(map[string]interface{}); ok {
		if isp, ok := traits["isp"].(string); ok {
			record.Traits.ISP = isp
		}
		if asn, ok := traits["autonomous_system_number"].(uint32); ok {
			record.Traits.AutonomousSystemNumber = uint(asn)
		}
		if asnOrg, ok := traits["autonomous_system_organization"].(string); ok {
			record.Traits.AutonomousSystemOrganization = asnOrg
		}
		if domain, ok := traits["domain"].(string); ok {
			record.Traits.Domain = domain
		}
		if connType, ok := traits["connection_type"].(string); ok {
			record.Traits.ConnectionType = connType
		}
		if userType, ok := traits["user_type"].(string); ok {
			record.Traits.UserType = userType
		}
	}
}

// readMetadata reads MMDB metadata
func (r *MMDBReader) readMetadata() (*MMDBMetadata, uint32, error) {
	// MMDB metadata is at the end of the file, preceded by a binary search tree marker
	fileInfo, err := r.file.Stat()
	if err != nil {
		return nil, 0, err
	}

	fileSize := fileInfo.Size()
	
	// Search backwards for metadata marker (0xAB 0xCD 0xEF MaxMind.DB)
	marker := []byte{0xAB, 0xCD, 0xEF, 0x4D, 0x61, 0x78, 0x4D, 0x69, 0x6E, 0x64, 0x2E, 0x63, 0x6F, 0x6D}
	markerLen := len(marker)
	
	var metadataOffset int64 = -1
	buf := make([]byte, 4096)
	
	for offset := fileSize - int64(markerLen) - 128*1024; offset >= 0 && offset >= fileSize-128*1024; offset-- {
		r.file.ReadAt(buf, offset)
		for i := 0; i <= len(buf)-markerLen; i++ {
			if string(buf[i:i+markerLen]) == string(marker) {
				metadataOffset = offset + int64(i) + int64(markerLen)
				break
			}
		}
		if metadataOffset != -1 {
			break
		}
	}
	
	if metadataOffset == -1 {
		return nil, 0, fmt.Errorf("metadata marker not found")
	}
	
	r.file.Seek(metadataOffset, 0)
	
	metadata := &MMDBMetadata{
		Description: make(map[string]string),
	}
	
	// Read metadata (simplified - actual format is more complex)
	binary.Read(r.file, binary.BigEndian, &metadata.BinaryFormatMajorVersion)
	binary.Read(r.file, binary.BigEndian, &metadata.BinaryFormatMinorVersion)
	binary.Read(r.file, binary.BigEndian, &metadata.BuildEpoch)
	
	// Read database type
	typeLen := make([]byte, 1)
	r.file.Read(typeLen)
	dbType := make([]byte, typeLen[0])
	r.file.Read(dbType)
	metadata.DatabaseType = string(dbType)
	
	// Read languages
	langCount := make([]byte, 1)
	r.file.Read(langCount)
	metadata.Languages = make([]string, langCount[0])
	for i := 0; i < int(langCount[0]); i++ {
		langLen := make([]byte, 1)
		r.file.Read(langLen)
		lang := make([]byte, langLen[0])
		r.file.Read(lang)
		metadata.Languages[i] = string(lang)
	}
	
	// Read record size and node count (simplified)
	binary.Read(r.file, binary.BigEndian, &metadata.RecordSize)
	binary.Read(r.file, binary.BigEndian, &metadata.NodeCount)
	binary.Read(r.file, binary.BigEndian, &metadata.IPVersion)
	
	// Calculate data section start (simplified)
	dataStart = uint32(metadata.NodeCount) * uint32(metadata.RecordSize) * 2
	
	return metadata, dataStart, nil
}

