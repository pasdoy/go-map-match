//api map match API and data parsing logic
package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

//Start start api
func Start() {
	log.SetLevel(log.DebugLevel)
	log.Info("Start API")

	r := gin.Default()
	r.GET("/match/:points", GetMatch)

	r.Run()
}

//GetMatch endpoint to snap data. Return list of points.
func GetMatch(c *gin.Context) {
	pointsParam := c.Param("points")
	rawPoints := strings.Split(pointsParam, ";")

	points := []Point{}
	for _, rawPoint := range rawPoints {
		rawPointSplit := strings.Split(rawPoint, ",")
		lat, _ := strconv.ParseFloat(rawPointSplit[0], 64) //TODO decide what format
		lon, _ := strconv.ParseFloat(rawPointSplit[1], 64)

		points = append(points, Point{Lat: lat, Lon: lon})
	}
	log.Debugf("Match input: %v", points)

	mapMaptch, err := MapMatch(points)
	if err != nil {
		log.Error(err)
		c.AbortWithStatus(400)
		return
	}

	//covert in format to debug [[lon, lat]]
	pts := make([][]float64, len(mapMaptch))
	for i, match := range mapMaptch {
		pts[i] = []float64{match.Lon, match.Lat}
	}

	c.JSON(200, pts)
}
