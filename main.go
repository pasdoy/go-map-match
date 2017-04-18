//main parses OSM data and starts the API
package main

import (
	"tmp/api"
)

func main() {
	api.ParseData()
	api.Start()
}
