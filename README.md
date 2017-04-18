# go-map-match
Simple, non-efficient, map matcher in go

## Usage
This is a dev lib. Do not use in production. I wanted to show how to use OSM data, build a rtree to do closest point search and road graph using go. It uses Djikstra to find the fastest path. 

The slowness comes, in part, from the huge road graph with all the OSM nodes. It is not efficient. It should be way segments only. This work is in progress.

## Install
1. `go get ./...`
1. Download `delaware-latest.osm.pbf` file from Geofabrik
1. `go run main.go`

## Use
```
curl 'http://localhost:8080/match/39.154015,-75.524993;39.154335,-75.526388;39.155399,-75.527152'
```

## Libs
- https://github.com/qedus/osmpbf
- https://github.com/dhconnelly/rtreego
- https://github.com/gyuho/goraph
- https://github.com/gin-gonic/gin