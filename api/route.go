package api

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/asaskevich/govalidator"
	"github.com/dhconnelly/rtreego"
	"github.com/gyuho/goraph"
	"github.com/kellydunn/golang-geo"
	"github.com/qedus/osmpbf"
	"io"
	"os"
	"runtime"
	"strconv"
	"time"
	"strings"
)

//Point struct
type Point struct {
	Lat float64
	Lon float64
}

//NearestNode return nearest node to point and distance
func (p Point) NearestNode() (node *Node, dist float64) {
	//TODO: add limit filter
	node = rt.NearestNeighbor(rtreego.Point{p.Lat, p.Lon}).(*Node)
	return node, p.Distance(node.Point)
}

//Distance between 2 points
func (p *Point) Distance(p2 Point) float64 {
	return geo.NewPoint(p.Lat, p.Lon).GreatCircleDistance(geo.NewPoint(p2.Lat, p2.Lon))
}

//NodeID id of a node
type NodeID string

func (id NodeID) String() string {
	return string(id) //strconv.FormatInt(int64(id), 10)
}

//Node local node
type Node struct {
	ID    goraph.StringID
	Point Point
	Tags  map[string]string
	Ways  []*Way
}

//Bounds to satisfy rtree interface
func (n *Node) Bounds() *rtreego.Rect {
	return rtreego.Point{n.Point.Lat, n.Point.Lon}.ToRect(0.0000001)
}

//Distance between 2 nodes
func (n *Node) Distance(n2 *Node) float64 {
	return geo.NewPoint(n.Point.Lat, n.Point.Lon).GreatCircleDistance(geo.NewPoint(n2.Point.Lat, n2.Point.Lon))
}

var nodes []*Node                            //all nodes read
var nodesIndex = map[goraph.StringID]*Node{} //index nodeID to node

//Way local way
type Way struct {
	ID      int64
	NodeIDs []NodeID
	Tags    map[string]string
	Nodes   []*Node
	Dist    float64
	MaxSpeed int
}

var ways []*Way //all ways read

var rt = rtreego.NewTree(2, 25, 50) //spatial index
var nodeGraph = goraph.NewGraph()   //road graph

//tags accepted as drivable ways
var VALID_WAYS = []string{
	"motorway",
	"trunk",
	"primary",
	"secondary",
	"tertiary",
	//"unclassified",
	"residential",
	"service",

	"motorway_link",
	"trunk_link",
	"primary_link",
	"secondary_link",
	"tertiary_link",
}

//default speed (mph) per road type
var WAY_SPEED = map[string]int{
	"motorway": 55,
	"trunk": 55,
	"primary": 35,
	"secondary": 25,
	"tertiary": 25,
	"service": 15,
	"residential": 25,
	"motorway_link": 55,
	"trunk_link": 55,
	"primary_link": 35,
	"secondary_link": 25,
	"tertiary_link": 25,
}

//return speed for way
func getWaySpeed(wayType, waySpeed string) int {
	wayType = strings.Replace(wayType, " mph", "", 1)
	speed, err := strconv.Atoi(waySpeed)
	if err != nil {

		if _, ok := WAY_SPEED[wayType]; !ok {
			log.Errorf("Missing speed default value for %s", wayType)
			return 25
		}

		return WAY_SPEED[wayType]
	}
	
	return speed
}

//ParseData load the data in cache (ram) from OSM
func ParseData() {
	log.Info("Parse OSM data")
	dur := time.Now()

	//read osm file
	f, err := os.Open("delaware-latest.osm.pbf")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	d := osmpbf.NewDecoder(f)

	// use more memory from the start, it is faster
	d.SetBufferSize(osmpbf.MaxBlobSize)

	// start decoding with several goroutines, it is faster
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		log.Fatal(err)
	}

	var nc, wc, rc uint64
	for {
		if v, err := d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		} else {
			switch v := v.(type) {
			case *osmpbf.Node:
				node := &Node{
					ID:    goraph.StringID(strconv.FormatInt(v.ID, 10)),
					Point: Point{Lat: v.Lat, Lon: v.Lon},
					Tags:  v.Tags}

				nodes = append(nodes, node)
				nodesIndex[node.ID] = node
				nc++
			case *osmpbf.Way:
				//filter to keep only "car" ways
				if !govalidator.IsIn(v.Tags["highway"], VALID_WAYS...) {
					continue
				}

				//remove ways with one node
				if len(v.NodeIDs) < 2 {
					continue
				}

				//r, _ := json.Marshal(v)
				//log.Info(string(r))
				ids := []NodeID{}
				for _, nodeID := range v.NodeIDs {
					ids = append(ids, NodeID(strconv.FormatInt(nodeID, 10)))
				}

				speed := getWaySpeed(v.Tags["highway"], v.Tags["maxspeed"])

				ways = append(ways, &Way{
					ID:      v.ID,
					NodeIDs: ids,
					Tags:    v.Tags,
					MaxSpeed: speed})
				// Process Way v.
				wc++
			case *osmpbf.Relation:
				// Process Relation v.
				continue
				rc++
			default:
				log.Fatalf("unknown type %T\n", v)
			}
		}
	}

	log.Infof("Nodes: %d, Ways: %d, Relations: %d", nc, wc, rc)
	log.Infof("Duration: %v", time.Since(dur))

	log.Info("Start indexing")
	dur = time.Now()

	for _, way := range ways {
		//append way pointer to nodes
		for _, nodeID := range way.NodeIDs {
			node := nodesIndex[goraph.StringID(nodeID)]
			way.Nodes = append(way.Nodes, node)
			node.Ways = append(node.Ways, way)
			rt.Insert(node)
		}

		//calc way distance in km
		for i := 0; i < len(way.Nodes)-1; i++ {
			curNode := way.Nodes[i]
			nextNode := way.Nodes[i+1]
			way.Dist  += curNode.Distance(nextNode)
		}

		if way.Dist == 0 {
			log.Warning(way)
		}

	}

	log.Infof("Rtree size: %i", rt.Size())
	log.Infof("Duration: %v", time.Since(dur))

	dur = time.Now()
	for _, way := range ways {
		//calc way distance in km + build graph of nodes from intersection only (when len(nextNode.Ways) > 1)
		firstNode := way.Nodes[0]
		curNode := firstNode
		segmentDist := 0.0
		for i := 1; i < len(way.Nodes); i++ {
			nextNode := way.Nodes[i]
			segmentDist += way.Nodes[i-1].Distance(nextNode)

			if len(nextNode.Ways) > 0 {
				//this is an intersection node
				nodeGraph.AddNode(goraph.NewNode(curNode.ID.String())) //will not overwrite if exists
				nodeGraph.AddNode(goraph.NewNode(nextNode.ID.String()))

				travelTime := segmentDist / float64(way.MaxSpeed)
				if travelTime == 0 {
					log.Error(segmentDist, way.MaxSpeed, travelTime)
					continue
				}

				nodeGraph.AddEdge(curNode.ID, nextNode.ID, travelTime)
				if _, ok := way.Tags["oneway"]; !ok {
					//Not a one way
					nodeGraph.AddEdge(nextNode.ID, curNode.ID, travelTime)
				}
				segmentDist = 0
				curNode = nextNode
			}

		}

		if firstNode == curNode {
			log.Debug("What??? a road with no end?") //TODO What are those?
		}

	}

	log.Infof("Graph size: %i", nodeGraph.GetNodeCount())
	log.Infof("Duration: %v", time.Since(dur))
}

//MapMatch a serie of Points to closest points and draw traject. If there's more than one point, repeat the process.
func MapMatch(points []Point) ([]Point, error) {
	if len(points) < 2 {
		return nil, errors.New("Need 2 points at least")
	}

	//get first node
	originNode, _ := points[0].NearestNode()
	if _, err := nodeGraph.GetNode(originNode.ID); err != nil {
		log.Panic("OriginNode doesn't exsists in graph")
		return nil, err
	}
	log.Debug(originNode.ID)

	matched := []Point{}

	//run Dijkstra between each node
	var destNodes *Node
	for i := 1; i < len(points); i++ {
		destNodes, _ = points[i].NearestNode()
		log.Debug(destNodes.ID)
		if _, err := nodeGraph.GetNode(destNodes.ID); err != nil {
			log.Panic("DestNode doesn't exsists in graph")
			return nil, err
		}

		ids, _, err := goraph.Dijkstra(nodeGraph, originNode.ID, destNodes.ID)
		if err != nil {
			return nil, err
		}

		log.Debug(ids)
		for _, nodeID := range ids {
			//build the roads
			node := nodesIndex[goraph.StringID(nodeID.String())]
			matched = append(matched, node.Point)
		}
		originNode = destNodes
	}

	return matched, nil
}
