package prices

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
)

// the format of this is based on the output format of https://github.com/erans/ec2instancespricing
type prices struct {
	Config struct {
		Currency string `json:"currency"`
		Unit     string `json:"unit"`
	} `json:"config"`
	Regions []struct {
		Instancetypes []struct {
			Os    string  `json:"os"`
			Price float64 `json:"price"`
			Type  string  `json:"type"`
		} `json:"instanceTypes"`
		Region string `json:"region"`
	} `json:"regions"`
}

type PricesMap map[string]map[string]float64

func getPricesMap(r io.Reader) (PricesMap, error) {
	var pricesMap PricesMap
	var prices prices
	err := json.NewDecoder(r).Decode(&prices)
	if err != nil {
		// pricesMap is nil
		return pricesMap, err
	}

	pricesMap = make(PricesMap)
	for _, region := range prices.Regions {
		// make all region maps
		pricesMap[region.Region] = make(map[string]float64)
		for _, instanceType := range region.Instancetypes {
			pricesMap[region.Region][instanceType.Type] = instanceType.Price
		}
	}
	return pricesMap, nil
}

func GetPricesMapFromFile(filename string) (PricesMap, error) {
	if filename == "" {
		return PricesMap{}, nil
	}
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return getPricesMap(bytes.NewReader(bs))
}
