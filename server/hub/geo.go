package hub

import (
	"log"
	"net"
	"github.com/oschwald/geoip2-golang"
)

var geoDB *geoip2.Reader

func InitGeo(dbPath string) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Printf("[Geo] Failed to open GeoLite2-City: %v. Map coords will be zeroed.", err)
		return
	}
	geoDB = db
	log.Println("[Geo] GeoLite2-City database loaded.")
}

func ResolveCoords(ip string) (float64, float64) {
	if geoDB == nil {
		return 0, 0
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return 0, 0
	}
	city, err := geoDB.City(parsedIP)
	if err != nil {
		return 0, 0
	}
	return city.Location.Latitude, city.Location.Longitude
}

// MapEvent is pushed to the live map stream
type MapEvent struct {
	Type     string  `json:"type"`
	DeviceID string  `json:"device_id"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Country  string  `json:"country"`
	Tier     string  `json:"tier"`
}
