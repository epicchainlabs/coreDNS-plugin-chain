# testdata
This directory contains mmdb database files used during the testing of this plugin.

# Create mmdb database files
If you need to change them to add a new value, or field the best is to recreate them, the code snipped used to create them initially is provided next.

```golang
package main

import (
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/inserter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

type location struct {
	Country   string  `json:"country"`
	CIDR      string  `json:"cidr"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func main() {
	file, err := os.Open("db.json")
	if err != nil {
		log.Fatal(err)
	}

	var locations []location

	if err = json.NewDecoder(file).Decode(&locations); err != nil {
		log.Fatal(err)
	}

	createCityDB("GeoIP2-City-Test.mmdb", "DBIP-City-Lite", locations)
}

func createCityDB(dbName, dbType string, locations []location) {
	// Load a database writer.
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: dbType})
	if err != nil {
		log.Fatal(err)
	}

	for _, loc := range locations {
		_, ip, err := net.ParseCIDR(loc.CIDR)
		if err != nil {
			log.Fatal(err)
		}

		record := mmdbtype.Map{
			"location": mmdbtype.Map{
				"accuracy_radius": mmdbtype.Uint16(100),
				"latitude":        mmdbtype.Float64(loc.Latitude),
				"longitude":       mmdbtype.Float64(loc.Longitude),
				"metro_code":      mmdbtype.Uint64(0),
				"time_zone":       mmdbtype.String("time zone"),
			},
		}

		if err := writer.InsertFunc(ip, inserter.TopLevelMergeWith(record)); err != nil {
			log.Fatal(err)
		}
	}

	// Write the DB to the filesystem.
	fh, err := os.Create(dbName)
	if err != nil {
		log.Fatal(err)
	}
	_, err = writer.WriteTo(fh)
	if err != nil {
		log.Fatal(err)
	}
}
```
