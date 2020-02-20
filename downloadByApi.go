package iotmaker_geo_pbf_import

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	kOsmApi06UrlNode string = "https://www.openstreetmap.org/api/0.6/node/"
	kOsmApi06UrlWay  string = "https://www.openstreetmap.org/api/0.6/way/"
)

type NodesTagStt struct {
	XMLName   xml.Name     `xml:"node"`
	Id        int64        `xml:"id,attr"`
	Lat       float64      `xml:"lat,attr"`
	Lon       float64      `xml:"lon,attr"`
	Version   int64        `xml:"version,attr"`
	TimeStamp string       `xml:"timestamp,attr"`
	ChangeSet string       `xml:"changeset,attr"`
	UId       int64        `xml:"uid,attr"`
	User      string       `xml:"user,attr"`
	Tag       []TagsTagStt `xml:"tag"`
}

type TagsTagStt struct {
	XMLName xml.Name `xml:"tag"`
	Key     string   `xml:"k,attr"`
	Value   string   `xml:"v,attr"`
}

type OsmNodeTagStt struct {
	XMLName   xml.Name    `xml:"osm"`
	Version   string      `xml:"version,attr"`
	Generator string      `xml:"generator,attr"`
	TimeStamp string      `xml:"timestamp,attr"`
	Node      NodesTagStt `xml:"node"`
}

type OsmWayTagStt struct {
	XMLName   xml.Name   `xml:"osm"`
	Version   string     `xml:"version,attr"`
	Generator string     `xml:"generator,attr"`
	TimeStamp string     `xml:"timestamp,attr"`
	Way       WaysTagStt `xml:"way"`
}

// Way tag from osm xml file
// Example: <way id="51402944" version="1" timestamp="2010-02-28T13:25:53Z" changeset="3997606" uid="31385" user="Skippern">
type WaysTagStt struct {
	XMLName   xml.Name         `xml:"way"`
	Id        int64            `xml:"id,attr"`
	Version   int64            `xml:"version,attr"`
	TimeStamp time.Time        `xml:"timestamp,attr"`
	ChangeSet int64            `xml:"changeset,attr"`
	UId       int64            `xml:"uid,attr"`
	User      string           `xml:"user,attr"`
	Tag       []TagsTagStt     `xml:"tag"`
	Ref       []NodesRefTagStt `xml:"nd"`
	Loc       [][2]float64     `xml:"-"`
	Rad       [][2]float64     `xml:"-"`
}

type NodesRefTagStt struct {
	XMLName xml.Name `xml:"nd"`
	Ref     int64    `xml:"ref,attr"`
}

// faz o download das informações de um node
func getNodeByApiOsm(idNode int64) (error, NodesTagStt) {
	nodeRemote := OsmNodeTagStt{}

	resp, err := http.Get(kOsmApi06UrlNode + strconv.FormatInt(idNode, 10))
	if err != nil {
		fmt.Printf("getNodeByApiOsm.http.Get.error: %v", err)
		return err, NodesTagStt{}
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	err = xml.Unmarshal(body, &nodeRemote)
	if err != nil {
		fmt.Printf("getNodeByApiOsm.xml.Unmarshal.error: %v\n", err)
		return err, NodesTagStt{}
	}

	if nodeRemote.Node.Lat == 0.0 && nodeRemote.Node.Lon == 0.0 {
		fmt.Printf("getNodeByApiOsm.nodeRemote.Lat == 0.0 && getNodeByApiOsm.nodeRemote.Lon == 0.0\n")
		return err, NodesTagStt{}
	}

	return nil, nodeRemote.Node
}

// faz o download das informações de um node
func getWayByApiOsm(idNode int64) (error, WaysTagStt) {
	wayRemote := OsmWayTagStt{}

	resp, err := http.Get(kOsmApi06UrlWay + strconv.FormatInt(idNode, 10))
	if err != nil {
		fmt.Printf("getWayByApiOsm.http.Get.error: %v", err)
		return err, WaysTagStt{}
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	err = xml.Unmarshal(body, &wayRemote)
	if err != nil {
		fmt.Printf("getWayByApiOsm.xml.Unmarshal.error: %v\n", err)
		return err, WaysTagStt{}
	}

	if len(wayRemote.Way.Ref) == 0 {
		fmt.Printf("getWayByApiOsm.len.wayRemote.Way.Loc == 0\n")
		return err, WaysTagStt{}
	}

	wayRemote.Way.Loc = make([][2]float64, len(wayRemote.Way.Ref))

	for k, idXmlNode := range wayRemote.Way.Ref {
		err, node := getNodeByApiOsm(idXmlNode.Ref)
		if err != nil {
			return errors.New("error downloading way id: " + strconv.FormatInt(idNode, 10)), WaysTagStt{}
		}

		wayRemote.Way.Loc[k] = [2]float64{node.Lon, node.Lat}
	}

	return nil, wayRemote.Way
}
