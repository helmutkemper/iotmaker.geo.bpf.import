package iotmaker_geo_pbf_import

import (
	"fmt"
	"github.com/helmutkemper/util"
	"os"
)

func ExampleImport_FindLonLatByIdInFile() {
	var lat, lon float64
	var err error
	var latAdd float64 = 0.1234567
	var lonAdd float64 = 0.7654321

	importMap := Import{}
	for i := 1; i != 30000; i += 1 {
		err = importMap.AppendLonLatToFile("./bin", int64(i), float64(i)+lonAdd, float64(i)+latAdd)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}

	for i := 1; i != 30000; i += 1 {
		err, lon, lat = importMap.FindLonLatByIdInFile("./bin", int64(i))
		if err != nil {
			fmt.Printf("err: %v\n", err)
			break
		}

		lon = util.Round(lon, 0.5, 8.0)
		lat = util.Round(lat, 0.5, 8.0)

		if lon != util.Round(float64(i)+lonAdd, 0.5, 8.0) {
			fmt.Printf("lon error: %v\n", i)
			fmt.Printf("lon found: %v\n", lon)
			fmt.Printf("expected lon: %v\n", util.Round(float64(i)+lonAdd, 0.5, 8.0))
			break
		}

		if lat != util.Round(float64(i)+latAdd, 0.5, 8.0) {
			fmt.Printf("lat error: %v\n", i)
			fmt.Printf("lat found: %v\n", lat)
			fmt.Printf("expected lat: %v\n", util.Round(float64(i)+latAdd, 0.5, 8.0))
			break
		}
	}

	err = os.Remove("./bin")
	if err != nil {
		fmt.Printf("error: %v\n", err.Error())
	}

	fmt.Printf("end\n")

	// Output:
	// end
}
