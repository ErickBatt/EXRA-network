package middleware

import (
	"log"
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

var asnDB *geoip2.Reader

func init() {
	db, err := geoip2.Open("GeoLite2-ASN.mmdb")
	if err != nil {
		log.Printf("WARN: GeoLite2-ASN.mmdb not found. Real ASN lookups disabled. (%v)", err)
		return
	}
	asnDB = db
}

// LookupASN checks the local MaxMind database for the given IP address.
func LookupASN(ipStr string, fallbackOrg string) (asnOrg string, isResidential bool) {
	// Strip port if present
	host, _, err := net.SplitHostPort(ipStr)
	if err != nil {
		host = ipStr
	}

	ip := net.ParseIP(host)

	// If DB is not available, or IP parsing failed, default to header heuristic
	if asnDB == nil || ip == nil {
		asnOrg = fallbackOrg
		if asnOrg == "" {
			asnOrg = "unknown"
		}
	} else {
		record, err := asnDB.ASN(ip)
		if err == nil {
			asnOrg = record.AutonomousSystemOrganization
		} else {
			asnOrg = fallbackOrg
		}
	}

	lowerASN := strings.ToLower(asnOrg)
	isResidential = true

	// Basic heuristic to catch datacenters
	datacenters := []string{"amazon", "aws", "digitalocean", "hetzner", "google", "cloud", "ovh", "linode", "vultr"}
	for _, dc := range datacenters {
		if strings.Contains(lowerASN, dc) {
			isResidential = false
			break
		}
	}

	return asnOrg, isResidential
}

