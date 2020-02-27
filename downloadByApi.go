package iotmaker_geo_pbf_import

import (
	"encoding/xml"
	"errors"
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
func DownloadNodeByApiOsm(idNode int64) (error, NodesTagStt) {
	var body []byte
	var err error
	var resp *http.Response

	nodeRemote := OsmNodeTagStt{}

	resp, err = http.Get(kOsmApi06UrlNode + strconv.FormatInt(idNode, 10))
	if err != nil {
		return err, NodesTagStt{}
	}

	body, err = ioutil.ReadAll(resp.Body)
	err = resp.Body.Close()
	if err != nil {
		return err, NodesTagStt{}
	}

	err = xml.Unmarshal(body, &nodeRemote)
	if err != nil {
		return err, NodesTagStt{}
	}

	if nodeRemote.Node.Lat == 0.0 && nodeRemote.Node.Lon == 0.0 {
		return errors.New("unknown error"), NodesTagStt{}
	}

	return nil, nodeRemote.Node
}

// faz o download das informações de um node
func DownloadWayByApiOsm(idNode int64) (error, WaysTagStt) {
	var err error
	var resp *http.Response
	var body []byte

	wayRemote := OsmWayTagStt{}

	resp, err = http.Get(kOsmApi06UrlWay + strconv.FormatInt(idNode, 10))
	if err != nil {
		return err, WaysTagStt{}
	}

	body, err = ioutil.ReadAll(resp.Body)
	err = resp.Body.Close()
	if err != nil {
		return err, WaysTagStt{}
	}

	err = xml.Unmarshal(body, &wayRemote)
	if err != nil {
		return err, WaysTagStt{}
	}

	if len(wayRemote.Way.Ref) == 0 {
		return err, WaysTagStt{}
	}

	wayRemote.Way.Loc = make([][2]float64, len(wayRemote.Way.Ref))

	for k, idXmlNode := range wayRemote.Way.Ref {
		err, node := DownloadNodeByApiOsm(idXmlNode.Ref)
		if err != nil {
			return err, WaysTagStt{}
		}

		wayRemote.Way.Loc[k] = [2]float64{node.Lon, node.Lat}
	}

	return nil, wayRemote.Way
}
