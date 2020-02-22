package iotmaker_geo_pbf_import

import "fmt"

func ExampleImport_FindLonLatByIdInFile() {
	var lat, lon float64
	var err error

	importMap := Import{}
	err = importMap.AppendNodeToFile("./bin", 1, 1.1, 1.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err = importMap.AppendNodeToFile("./bin", 2, 2.1, 2.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err = importMap.AppendNodeToFile("./bin", 3, 3.1, 3.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err = importMap.AppendNodeToFile("./bin", 4, 4.1, 4.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err = importMap.AppendNodeToFile("./bin", 5, 5.1, 5.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err = importMap.AppendNodeToFile("./bin", 6, 6.1, 6.2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 1)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 2)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 3)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 4)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 5)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 6)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	err, lon, lat = importMap.FindLonLatByIdInFile("./bin", 7)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	fmt.Printf("lon: %v\n", lon)
	fmt.Printf("lat: %v\n", lat)

	// Output:
	// lon: 1.1
	// lat: 1.2
	// lon: 2.1
	// lat: 2.2
	// lon: 3.1
	// lat: 3.2
	// lon: 4.1
	// lat: 4.2
	// lon: 5.1
	// lat: 5.2
	// lon: 6.1
	// lat: 6.2
	// err: id not found
	// lon: 0
	// lat: 0
}
