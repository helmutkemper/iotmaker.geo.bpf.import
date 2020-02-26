package iotmaker_geo_pbf_import

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/helmutkemper/osmpbf"
	"github.com/helmutkemper/util"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Import struct {
	nodesCount     int64
	waysCount      int64
	relationsCount int64
	othersCount    int64

	nodesFound     chan int64
	nodesIgnored   chan int64
	nodesProcessed chan int64

	mapFile            string
	dirFromBinaryFiles string

	unnecessaryTags map[string]string
}

func (el *Import) SetMapFilePath(path string) error {
	if !util.CheckFileOrDirExists(path) {
		return errors.New("map file not found")
	}

	el.mapFile = path
	return nil
}

func (el *Import) SetDirFromBinaryFilesCache(path string) error {
	if strings.HasSuffix(path, "/") {
		path += "/"
	}

	if !util.CheckFileOrDirExists(path) {
		return errors.New("binary dir not found")
	}

	el.dirFromBinaryFiles = path
	return nil
}

// en: Checks to see if the file maps and the directory of the binary files exists
//
// pt_br: Verifica se o arquivo de mapas e o diretório dos arquivos binários existem
func (el *Import) Verify() error {
	if el.mapFile == "" {
		return errors.New("please, set a map file path first")
	}

	if el.dirFromBinaryFiles == "" {
		return errors.New("please, set a dir from a binary files cache first")
	}

	return nil
}

func (el *Import) TagToDeleteAddKeyToDelete(key string) {
	if len(el.unnecessaryTags) == 0 {
		el.unnecessaryTags = make(map[string]string)
	}

	el.unnecessaryTags[key] = ""
}

func (el *Import) TagToDeleteAddKeyValueToDelete(key, value string) {
	if len(el.unnecessaryTags) == 0 {
		el.unnecessaryTags = make(map[string]string)
	}

	el.unnecessaryTags[key] = value
}

func (el *Import) deleteUnnecessaryTags(tag *map[string]string) {
	delete(*tag, "source")
	delete(*tag, "history")
	delete(*tag, "converted_by")
	delete(*tag, "created_by")
	delete(*tag, "wikipedia")

	if len(el.unnecessaryTags) == 0 {

		for tagInListKey, tagInListValue := range el.unnecessaryTags {

			if tagInListValue == "" {
				delete(*tag, tagInListKey)

			} else {

				for tagInWayKey, tagInWayValue := range *tag {
					if tagInWayKey == tagInListKey && tagInWayValue == tagInListValue {
						delete(*tag, tagInListKey)
						break

					}
				}
			}

		}
	}
}

func (el *Import) CountElements() error {
	var err error

	err = el.Verify()
	if err != nil {
		return err
	}

	var v interface{}
	var f *os.File

	f, err = os.Open(el.mapFile)
	if err != nil {
		return err
	}

	d := osmpbf.NewDecoder(f)
	d.SetBufferSize(osmpbf.MaxBlobSize)
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		return err
	}

	for {

		if v, err = d.Decode(); err == io.EOF {
			break

		} else if err != nil {
			return err

		} else {
			switch v.(type) {

			case *osmpbf.Node:
				el.nodesCount += 1

			case *osmpbf.Way:
				el.waysCount += 1

			case *osmpbf.Relation:
				el.relationsCount += 1

			default:
				el.othersCount += 1

			}
		}
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return nil
}

func (el *Import) MakeFileId(id, moduleValue int64) int64 {
	return id % moduleValue
}

func (el *Import) AppendNodeToFile(node *osmpbf.Node) error {
	return el.AppendLonLatToFile(node.ID, node.Lon, node.Lat)
}

func (el *Import) FileManagerFindLonLatInFile(idToFind int64) (error, float64, float64) {
	var err error
	var lon, lat float64

	err, lon, lat = el.FindLonLatByIdInFile(idToFind)
	return err, lon, lat
}

func (el *Import) findAndInsertWaysLonAndLat(nodesIds *[]int64, wayOut *WayConverted) error {
	var err error
	var lon, lat float64

	for _, idNode := range *nodesIds {
		err, lon, lat = el.FileManagerFindLonLatInFile(idNode)
		if err != nil {
			return err
		}

		wayOut.AddLonLat(lon, lat)
	}

	return nil
}

func (el *Import) PopulateWay(way *osmpbf.Way) (error, *WayConverted) {
	var err error
	var ret = NewWayConverted()

	el.deleteUnnecessaryTags(&way.Tags)
	err = el.findAndInsertWaysLonAndLat(&way.NodeIDs, ret)
	if err != nil {
		return err, nil
	}

	ret.CopyTags(&way.Tags)
	ret.AddInfo(&way.Info)
	ret.ID = way.ID

	return nil, ret
}

func (el *Import) FindAllNodesForTest() error {
	var err error

	err = el.Verify()
	if err != nil {
		return err
	}

	var v interface{}
	var f *os.File

	f, err = os.Open(el.mapFile)
	if err != nil {
		return err
	}
	defer f.Close()

	d := osmpbf.NewDecoder(f)
	d.SetBufferSize(osmpbf.MaxBlobSize)
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		return err
	}

	for {

		if v, err = d.Decode(); err == io.EOF {
			break

		} else if err != nil {
			return err

		} else {
			switch v.(type) {

			case *osmpbf.Node:

				err, _, _ = el.FileManagerFindLonLatInFile(v.(*osmpbf.Node).ID)
				if err != nil {
					return err
				}

			case *osmpbf.Way:
				break

			case *osmpbf.Relation:
				break

			}
		}
	}

	return nil
}

func (el *Import) ExtractNodesToBinaryFilesDir() error {
	var err error

	err = el.Verify()
	if err != nil {
		return err
	}

	var v interface{}
	var f *os.File

	el.nodesFound = make(chan int64, 1)
	el.nodesIgnored = make(chan int64, 1)
	el.nodesProcessed = make(chan int64, 1)

	f, err = os.Open(el.mapFile)
	if err != nil {
		return err
	}
	defer f.Close()

	d := osmpbf.NewDecoder(f)
	d.SetBufferSize(osmpbf.MaxBlobSize)
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		return err
	}

	for {

		if v, err = d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			return err
		} else {
			switch v.(type) {

			case *osmpbf.Node:

				err = el.AppendNodeToFile(v.(*osmpbf.Node))
				if err != nil {
					return err
				}
				continue

			case *osmpbf.Way:
				return nil

			case *osmpbf.Relation:
				return nil

			}
		}
	}

	return nil
}

func (el *Import) ProcessWaysFromMapFile(externalFunction func(wayConverted WayConverted)) error {
	var err error

	err = el.Verify()
	if err != nil {
		return err
	}

	if externalFunction == nil {
		return errors.New("please, set a external function to process all ways")
	}

	var v interface{}
	var f *os.File
	var wayConverted *WayConverted

	f, err = os.Open(el.mapFile)
	if err != nil {
		return err
	}
	defer f.Close()

	d := osmpbf.NewDecoder(f)
	d.SetBufferSize(osmpbf.MaxBlobSize)
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		return err
	}

	for {

		if v, err = d.Decode(); err == io.EOF {
			break

		} else if err != nil {
			return err

		} else {
			switch v.(type) {

			case *osmpbf.Node:
				continue

			case *osmpbf.Way:

				err, wayConverted = el.PopulateWay(v.(*osmpbf.Way))
				externalFunction(*wayConverted)

			case *osmpbf.Relation:
				return nil

			}
		}
	}

	return nil
}

func (el *Import) AppendLonLatToFile(idIn int64, lonIn, latIn float64) error {
	var err error
	var nodesFile *os.File
	var fileIn string

	if idIn == 0 || (lonIn == 0.0 && latIn == 0.0) {
		return errors.New("zero as value")
	}

	err, fileIn = el.applyRuleToBinaryFile(idIn)
	if err != nil {
		return err
	}

	err, _, _ = el.FindLonLatByIdInFile(idIn)
	if err == nil {
		return nil
	}

	nodesFile, err = os.OpenFile(fileIn, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer nodesFile.Close()

	bufWriter := new(bytes.Buffer)

	lonIn = util.Round(lonIn, 0.5, 8.0)
	latIn = util.Round(latIn, 0.5, 8.0)

	err = binary.Write(bufWriter, binary.BigEndian, idIn)
	if err != nil {
		return err
	}

	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		return err
	}

	bufWriter = new(bytes.Buffer)

	err = binary.Write(bufWriter, binary.BigEndian, lonIn)
	if err != nil {
		return err
	}

	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		return err
	}

	bufWriter = new(bytes.Buffer)

	err = binary.Write(bufWriter, binary.BigEndian, latIn)
	if err != nil {
		return err
	}
	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (el *Import) applyRuleToBinaryFile(id int64) (error, string) {
	var err error

	err = el.Verify()
	if err != nil {
		return err, ""
	}

	idFileInt64 := el.MakeFileId(id, 1000)
	idFileStr := strconv.FormatInt(idFileInt64, 10)
	filePath := el.dirFromBinaryFiles + idFileStr + ".bin"

	return nil, filePath
}

func (el *Import) FindLonLatByIdInFile(idToFind int64) (error, float64, float64) {
	var err error
	var nodesFile *os.File
	var fileIn string

	bufReader := &bytes.Reader{}

	idByte := make([]byte, 8)
	float64Byte := make([]byte, 8)

	var idInt64 int64
	var lonFloat64, latFloat64 float64

	err, fileIn = el.applyRuleToBinaryFile(idToFind)
	if err != nil {
		return err, 0.0, 0.0
	}

	nodesFile, err = os.OpenFile(fileIn, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err, 0.0, 0.0
	}

	defer nodesFile.Close()

	var filePointer int64 = 0

	for {
		// read node id
		_, err = nodesFile.ReadAt(idByte, filePointer)
		if err == io.EOF {
			nodesFile.Close()
			break
		}

		filePointer += 8

		bufReader = bytes.NewReader(idByte)
		err = binary.Read(bufReader, binary.BigEndian, &idInt64)
		if err != nil {
			return err, 0.0, 0.0
		}

		if idInt64 != idToFind {
			// lon e lat pointers
			filePointer += 8 + 8
			continue
		}

		// read node lon
		_, err = nodesFile.ReadAt(float64Byte, filePointer)
		if err != nil {
			return err, 0.0, 0.0
		}

		filePointer += 8

		bufReader = bytes.NewReader(float64Byte)
		err = binary.Read(bufReader, binary.BigEndian, &lonFloat64)
		if err != nil {
			return err, 0.0, 0.0
		}

		// read node lat
		_, err = nodesFile.ReadAt(float64Byte, filePointer)
		if err != nil {
			return err, 0.0, 0.0
		}

		filePointer += 8

		bufReader = bytes.NewReader(float64Byte)
		err = binary.Read(bufReader, binary.BigEndian, &latFloat64)
		if err != nil {
			return err, 0.0, 0.0
		}

		return nil, lonFloat64, latFloat64
	}

	return errors.New("id not found"), 0.0, 0.0
}

type WayConverted struct {
	ID   int64
	Node [][2]float64
	Tags map[string]string
	Info osmpbf.Info
}

func (el *WayConverted) AddLonLat(lon, lat float64) {
	el.Node = append(el.Node, [2]float64{lon, lat})
}

func (el *WayConverted) AddTag(key, value string) {
	if len(el.Tags) == 0 {
		el.Tags = make(map[string]string)
	}
	el.Tags[key] = value
}

func (el *WayConverted) AddInfo(info *osmpbf.Info) {
	el.Info.Changeset = info.Changeset
	el.Info.Timestamp = info.Timestamp
	el.Info.Uid = info.Uid
	el.Info.Version = info.Version
	el.Info.Visible = info.Visible
}

func (el *WayConverted) CopyTags(originalList *map[string]string) {
	el.Tags = make(map[string]string)

	for key, value := range *originalList {
		el.Tags[key] = value
	}
}

func NewWayConverted() *WayConverted {
	newWay := WayConverted{}
	newWay.Node = make([][2]float64, 0)
	newWay.Tags = make(map[string]string)
	newWay.Info = osmpbf.Info{}

	return &newWay
}

type wayError struct {
	Id        int64
	Processed bool
}

var (
//configSkipExistentData    = false
//configNodesPerInteraction = 100
//nodesSearchCount          = 0

//nodesProcessMapGlobalTotal = 0

//waysInteractionCount = 0
//waysListLineCount    = 0
//waysCount            = 0

//notFoundCount = 0

//radixTreeWays *radix.Tree

//waysListProcessed = make([]iotmaker_geo_osm.WayStt, configNodesPerInteraction)
//waysList          = make([]osmpbf.Way, configNodesPerInteraction)
)

type _NodeDb struct {
	Index string
	Loc   [2]float64
}

type _Way struct {
	ID      int64
	Tags    map[string]string
	NodeIDs []int64
	Info    osmpbf.Info
}

type _ToJSonCache struct {
	ID  int64
	Lat float64
	Lon float64
}
