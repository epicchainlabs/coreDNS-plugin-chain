package geodns

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

type db struct {
	readers map[int]*geoip2.Reader
	m       sync.RWMutex
}

const (
	isCity = 1 << iota
	isCountry
)

var probingIP = net.ParseIP("127.0.0.1")

func typeToString(dbType int) string {
	switch dbType {
	case isCity:
		return "city"
	case isCountry:
		return "country"
	}

	return fmt.Sprintf("unkonwn type %d", dbType)
}

func (db *db) AddReader(dbType int, r *geoip2.Reader) {
	db.m.Lock()
	db.readers[dbType] = r
	db.m.Unlock()
}

func (db *db) Reader(dbType int) (*geoip2.Reader, error) {
	db.m.RLock()
	defer db.m.RUnlock()

	r, ok := db.readers[dbType]
	if !ok {
		return nil, fmt.Errorf("db with type %d not found", dbType)
	}
	return r, nil
}

type IPInformation struct {
	City    *geoip2.City
	Country *geoip2.Country
}

type DistanceInfo struct {
	Distance       float64
	CountryMatched bool
}

func (i *IPInformation) IsEmpty() bool {
	if i.City == nil && i.Country == nil {
		return true
	}

	if i.City != nil && i.City.Location == emptyLocation.Location && i.Country == nil {
		return true
	}

	return false
}

func (db *db) IPInfo(ip net.IP) *IPInformation {
	result := &IPInformation{}

	cityDB, err := db.Reader(isCity)
	if err == nil {
		city, err := cityDB.City(ip)
		if err != nil {
			log.Debugf("couldn't get data from city db: %s", err.Error())
		} else {
			result.City = city

		}
	}

	countryDB, err := db.Reader(isCountry)
	if err == nil {
		country, err := countryDB.Country(ip)
		if err != nil {
			log.Debugf("couldn't get data from country db: %s", err.Error())
		} else {
			result.Country = country
		}
	}

	return result
}

// we have to check type because geoip2 lib allows city request to country db for backward compatibility.
func getDBType(r *geoip2.Reader) (int, error) {
	switch r.Metadata().DatabaseType {
	case "DBIP-City-Lite",
		"DBIP-Location (compat=City)",
		"GeoLite2-City",
		"GeoIP2-City",
		"GeoIP2-City-Africa",
		"GeoIP2-City-Asia-Pacific",
		"GeoIP2-City-Europe",
		"GeoIP2-City-North-America",
		"GeoIP2-City-South-America",
		"GeoIP2-Precision-City":
		return isCity, nil
	case "GeoLite2-Country",
		"GeoIP2-Country",
		"DBIP-Country-Lite",
		"DBIP-Country":
		return isCountry, nil
	}

	return 0, fmt.Errorf("unkonwn db type: %s", r.Metadata().DatabaseType)
}
