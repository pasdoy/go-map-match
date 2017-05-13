//main parses OSM data and starts the API
package main

import (
	"go-map-match/api"
)

func main() {
	api.ParseData()
	api.Start()
}
