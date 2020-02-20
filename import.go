package iotmaker_geo_pbf_import

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/helmutkemper/gOsm/geoMath"
	"github.com/helmutkemper/gOsm/utilMath"
	"github.com/helmutkemper/go-radix"
	iotmaker_db_interface "github.com/helmutkemper/iotmaker.db.interface"
	"github.com/helmutkemper/mongodb"
	"github.com/helmutkemper/osmpbf"
	log "github.com/helmutkemper/seelog"
	"github.com/helmutkemper/util"
	"go.mongodb.org/mongo-driver/bson"
	"io"
	"os"
	"runtime"
	"strconv"
)

type wayError struct {
	Id        int64
	Processed bool
}

type wayConverted struct {
	ID   int64
	Node [][2]float64
	Tags map[string]string
	Info osmpbf.Info
}

var (
	configSkipExistentData    = true
	configNodesPerInteraction = 1000000
	nodesSearchCount          = 0

	nodesProcessMapGlobalTotal = 0

	waysInteractionCount = 0
	waysListLineCount    = 0
	waysCount            = 0

	notFoundCount = 0

	radixTreeWays *radix.Tree

	waysListProcessed = make([]geoMath.WayStt, configNodesPerInteraction)
	waysList          = make([]osmpbf.Way, configNodesPerInteraction)
)

type NodeDb struct {
	Index string
	Loc   [2]float64
}

type Way struct {
	ID      int64
	Tags    map[string]string
	NodeIDs []int64
	Info    osmpbf.Info
}

func ProcessPbfFileInMemory(db iotmaker_db_interface.DbFunctionsInterface, mapFile, tmpFile string) {

	var v interface{}
	var totalFromQuery int64

	f, err := os.Open(mapFile)
	if err != nil {
		_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.os.Open.error: %v", err)
	}

	d := osmpbf.NewDecoder(f)

	// use more memory from the start, it is faster
	d.SetBufferSize(osmpbf.MaxBlobSize)

	// start decoding with several goroutines, it is faster
	err = d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.d.Start.error: %v", err)
	}

	for {

		if v, err = d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.d.Decode.error: %v", err)
		} else {
			switch v.(type) {

			case *osmpbf.Node:

				node := v.(*osmpbf.Node)

				if TestTagNodeToDiscard(&node.Tags) == true {
					continue
				}

				if configSkipExistentData == true {
					err, totalFromQuery = db.Count("point", bson.M{"id": v.(*osmpbf.Node).ID})
					if err != nil {
						_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.db.point.count.error: %v", err)
					}

					if totalFromQuery != 0 {
						continue
					}
				}

				point := geoMath.PointStt{}
				err = point.SetLngLatDegrees(node.Lon, node.Lat)
				if err != nil {
					_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.SetLngLatDegrees.error: %v", err)
				}
				point.Tag = node.Tags
				point.UId = int64(node.Info.Uid)
				point.ChangeSet = node.Info.Changeset
				point.User = node.Info.User
				point.TimeStamp = node.Info.Timestamp
				point.Version = int64(node.Info.Version)
				point.Visible = node.Info.Visible
				point.Id = node.ID
				point.MakeGeoJSonFeature()
				point.Prepare()
				err, _ = point.MakeMD5()
				if err != nil {
					_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.MakeMD5.error: %v", err)
				}

				err = db.Insert("point", point)
				if err != nil {
					_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.insert.error: %v", err)
				}

			case *osmpbf.Way:

				way := v.(*osmpbf.Way)

				if configSkipExistentData == true {
					err, totalFromQuery = db.Count("way", bson.M{"id": v.(*osmpbf.Way).ID})
					if err != nil {
						_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.db.way.count.error: %v", err)
					}

					if totalFromQuery != 0 {
						continue
					}
				}

				if waysInteractionCount >= configNodesPerInteraction {
					process(tmpFile)
					populate(tmpFile)
					waysListLineCount = 0
					waysInteractionCount = 0
				}

				if waysInteractionCount == 0 {
					nodesSearchCount = 0
					radixTreeWays = radix.New()
				}

				waysList[waysListLineCount] = *way
				waysListProcessed[waysListLineCount] = geoMath.WayStt{
					Id:        way.ID,
					Loc:       make([][2]float64, len(way.NodeIDs)),
					Rad:       make([][2]float64, len(way.NodeIDs)),
					Tag:       way.Tags,
					Data:      make(map[string]string),
					UId:       int64(way.Info.Uid),
					ChangeSet: way.Info.Changeset,
					User:      way.Info.User,
					TimeStamp: way.Info.Timestamp,
					Version:   int64(way.Info.Version),
					Visible:   way.Info.Visible,
				}

				for _, idNode := range way.NodeIDs {
					nodesSearchCount += 1
					idNodeString := strconv.FormatInt(idNode, 10)
					_, found := radixTreeWays.Get(idNodeString)
					if found == false {
						radixTreeWays.Insert(idNodeString, [2]float64{0.0, 0.0})
					}
				}

				waysCount += 1
				waysListLineCount += 1
				waysInteractionCount += 1

			case *osmpbf.Relation:

				//relation := v.(*osmpbf.Relation)

				if waysInteractionCount != 0 {
					waysInteractionCount = 0
					process(tmpFile)
					populate(tmpFile)
				}

			default:
				_ = log.Error("unknown type %T\n", v)
			}
		}
	}

	err = f.Close()
	if err != nil {
		_ = log.Errorf("gosmImport.ProcessPbfFileInMemory.f.close.error: %v", err)
	}
}

func TestTagNodeToDiscard(tag *map[string]string) bool {

	deleteTagsUnnecessary(tag)

	length := len(*tag)
	count := 0

	for k := range *tag {
		switch k {
		case "building":
			count += 1
		}
	}

	if length == count {
		return true
	}

	if len(*tag) == 0 {
		return true
	}

	return false
}

func deleteTagsUnnecessary(tag *map[string]string) {
	delete(*tag, "source")
	delete(*tag, "history")
	delete(*tag, "converted_by")
	delete(*tag, "created_by")
	delete(*tag, "wikipedia")
}

// pt_br: Adiciona osm.id, longitude e latitude em um arquivo binário, afim de fazer
// uma busca em memória mais rápida do que em banco de dados.
//
// O maior problema da importação de arquivos é o excesso de pontos usados para
// formar todos os outros tipos de dados.
// Eles fazem bancos como o mongoDB ficarem muitos lentos a medidas que um volume
// muito grande de dados é inserido.
func AppendNodeToFile(outputFile string, idIn int64, lonIn, latIn float64) error {

	nodesFile, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.os.OpenFile.error: %v", err.Error())
		return err
	}
	defer nodesFile.Close()

	bufWriter := new(bytes.Buffer)

	lonIn = util.Round(lonIn, 0.5, 8.0)
	latIn = util.Round(latIn, 0.5, 8.0)

	err = binary.Write(bufWriter, binary.BigEndian, idIn)
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.binary.Write.error: %v", err.Error())
		return err
	}

	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.nodesFile.Write.error: %v", err.Error())
		return err
	}

	bufWriter = new(bytes.Buffer)

	err = binary.Write(bufWriter, binary.BigEndian, lonIn)
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.binary.Write.error: %v", err.Error())
		return err
	}

	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.nodesFile.Write.error: %v", err.Error())
		return err
	}

	bufWriter = new(bytes.Buffer)

	err = binary.Write(bufWriter, binary.BigEndian, latIn)
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.binary.Write.error: %v", err.Error())
		return err
	}
	_, err = nodesFile.Write(bufWriter.Bytes())
	if err != nil {
		_ = log.Errorf("AppendNodeToFile.nodesFile.Write.error: %v", err.Error())
		return err
	}

	return nil
}

func process(inputFile string) {
	bufReader := &bytes.Reader{}

	idByte := make([]byte, 8)
	float64Byte := make([]byte, 8)

	var idInt64 int64
	var lonFloat64, latFloat64 float64

	nodesFile, err := os.OpenFile(inputFile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		_ = log.Errorf("gosmImport.process.os.OpenFile.error: %v", err)
	}

	var filePointer int64 = 0

	for {
		nodesProcessMapGlobalTotal += 1

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
			_ = log.Errorf("gosmImport.process.bytes.NewReader.error: %v", err)
		}

		// read node lon
		_, err = nodesFile.ReadAt(float64Byte, filePointer)
		if err != nil {
			_ = log.Errorf("gosmImport.process.nodesFile.ReadAt.error: %v", err)
		}

		filePointer += 8

		bufReader = bytes.NewReader(float64Byte)
		err = binary.Read(bufReader, binary.BigEndian, &lonFloat64)
		if err != nil {
			_ = log.Errorf("gosmImport.process.bytes.NewReader.error: %v", err)
		}

		// read node lat
		_, err = nodesFile.ReadAt(float64Byte, filePointer)
		if err != nil {
			_ = log.Errorf("gosmImport.process.nodesFile.ReadAt.error: %v", err)
		}

		filePointer += 8

		bufReader = bytes.NewReader(float64Byte)
		err = binary.Read(bufReader, binary.BigEndian, &latFloat64)
		if err != nil {
			_ = log.Errorf("gosmImport.process.bytes.NewReader.error: %v", err)
		}

		idNodeString := strconv.FormatInt(idInt64, 10)
		_, found := radixTreeWays.Get(idNodeString)
		if found == true {
			radixTreeWays.Insert(idNodeString, [2]float64{lonFloat64, latFloat64})
		}
	}

	nodesFile.Close()

	nodesProcessMapGlobalTotal = 0
}

func populate(db iotmaker_db_interface.DbFunctionsInterface, inputFile string) {

	for i := 0; i != 3; i += 1 {
		var allPass = true

		for listKey, wayP := range waysList {

			for nodeKey, idNode := range wayP.NodeIDs {

				idNodeString := strconv.FormatInt(idNode, 10)
				coordinate, found := radixTreeWays.Get(idNodeString)
				if found == true && coordinate.([2]float64)[0] != 0.0 && coordinate.([2]float64)[1] != 0.0 {
					waysListProcessed[listKey].Loc[nodeKey] = coordinate.([2]float64)
				} else {

					allPass = false
					err, coordinateFromServer := getNodeByApiOsm(idNode)
					if err != nil {

						// todo: arquivar em banco os errors na hora de pegar
						//waysListProcessed[listKey].Data["error"] = true

						//wayError := wayError{Id: waysListProcessed[listKey].Id}
						//query := mongodb.QueryStt{
						//  Query: bson.M{"id": wayError.Id},
						//}
						//if wayErrorToDb.Count(&query) == 0 {
						//  wayErrorToDb.Insert(wayError)
						//}
					}

					err = AppendNodeToFile(inputFile, coordinateFromServer.Id, coordinateFromServer.Lon, coordinateFromServer.Lat)
					if err != nil {
						_ = log.Errorf("gosmImport.populate.AppendNodeToFile.error: %v", err)
						continue
					}

					radixTreeWays.Insert(idNodeString, [2]float64{coordinateFromServer.Lon, coordinateFromServer.Lat})
					waysListProcessed[listKey].Loc[nodeKey] = [2]float64{coordinateFromServer.Lon, coordinateFromServer.Lat}
				}
			}
		}

		if allPass == true {
			break
		}
	}

	verify(db)
}

func verify(db iotmaker_db_interface.DbFunctionsInterface) {
	var err error

	for wayKey := range waysListProcessed {
		pass := true
		for k := range waysListProcessed[wayKey].Loc {
			if waysListProcessed[wayKey].Loc[k][0] == 0.0 && waysListProcessed[wayKey].Loc[k][1] == 0.0 {
				fmt.Printf("fixme: entrou. id: %v\n", waysList[wayKey].NodeIDs[k])

				err, wayTag := getWayByApiOsm(waysList[wayKey].ID)
				if err != nil {
					pass = false
				} else {

					waysListProcessed[wayKey].Tag = make(map[string]string)
					for _, v := range wayTag.Tag {
						waysListProcessed[wayKey].Tag[v.Key] = v.Value
					}

					waysListProcessed[wayKey].Loc = make([][2]float64, len(wayTag.Loc))
					waysListProcessed[wayKey].Rad = make([][2]float64, len(wayTag.Loc))

					for kNode, v := range wayTag.Loc {
						waysListProcessed[wayKey].Loc[kNode] = v
						waysListProcessed[wayKey].Rad[kNode] = [2]float64{utilMath.DegreesToRadians(v[0]), utilMath.DegreesToRadians(v[1])}
					}

				}

				break
			}
		}

		if pass == false {
			notFoundCount += 1
			fmt.Printf("way id: %v, not found\n", waysListProcessed[wayKey].Id)
		} else {

			geoMathWay := waysListProcessed[wayKey]

			geoMathWay.MakeGeoJSonFeature()
			geoMathWay.Prepare()
			geoMathWay.Init()
			geoMathWay.MakeMD5()

			err = db.Insert("way", geoMathWay)
			if err != nil {
				_ = log.Errorf("gosmImport.verify.geoMathWay.insert.error: %v", err)
			}

			if geoMathWay.Tag["type"] == "multipolygon" || geoMathWay.IsPolygon() == true {

				polygon := geoMath.PolygonStt{}
				polygon.Id = geoMathWay.Id
				polygon.Tag = geoMathWay.Tag
				polygon.UId = int64(geoMathWay.UId)
				polygon.ChangeSet = int64(geoMathWay.Changeset)
				polygon.User = geoMathWay.User
				polygon.TimeStamp = geoMathWay.TimeStamp
				polygon.Version = int64(geoMathWay.Version)
				polygon.Visible = geoMathWay.Visible
				polygon.Prepare()
				polygon.AddWayAsPolygon(&geoMathWay)
				polygon.MakeGeoJSonFeature()
				polygon.Init()
				polygon.MakeMD5()

				deleteTagsUnnecessary(&polygon.Tag)

				polygonToDb.Insert(polygon)
				err := polygonToDb.GetLastError()
				if err != nil {
					fmt.Printf("polygonToDb.Insert.Error: %v\n", err.Error())
				}
			}
		}
	}
}
