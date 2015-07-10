package prices

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/yosuke-furukawa/json5/encoding/json5"

	log "github.com/mozilla-services/reaper/reaperlog"
)

type prices struct {
	Vers   json5.Number `json:"vers,Number"`
	Config struct {
		Rate         string   `json:"rate"`
		ValueColumns []string `json:"valueColumns"`
		Currencies   []string `json:"currencies"`
		Regions      []struct {
			Region        string `json:"region"`
			InstanceTypes []struct {
				Type  string `json:"type"`
				Sizes []struct {
					Size         string `json:"size"`
					VCPU         string `json:"vCPU"`
					ECU          string `json:"ECU"`
					MemoryGiB    string `json:"memoryGiB"`
					StorageGB    string `json:"storageGB"`
					ValueColumns []struct {
						Name   string `json:"name"`
						Prices struct {
							USD string `json:"USD"`
						} `json:"prices"`
					} `json:"valueColumns"`
				} `json:"sizes"`
			} `json:"instanceTypes"`
		} `json:"regions"`
	} `json:"config"`
}

type pricesFromScript struct {
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

type PricesFromScriptMap map[string]map[string]float64

func GetPricesFromScriptMap() PricesFromScriptMap {
	b, err := ioutil.ReadFile("prices/prices.json")
	if err != nil {
		log.Error(fmt.Sprintf("%s", err))
	}
	var prices pricesFromScript
	err = json.Unmarshal(b, &prices)
	if err != nil {
		return PricesFromScriptMap{}
	}
	pricesMap := make(PricesFromScriptMap)
	for _, region := range prices.Regions {
		// make all region maps
		pricesMap[region.Region] = make(map[string]float64)
		for _, instanceType := range region.Instancetypes {
			pricesMap[region.Region][instanceType.Type] = instanceType.Price
		}
	}
	return pricesMap
}

// PricesMap is a map of Region -> Type -> Size -> Price
type PricesMap map[string]map[string]string

func GetPricesMap() PricesMap {
	prices := getPrices()
	pricesMap := make(map[string]map[string]string)
	for _, region := range prices.Config.Regions {
		// make all region maps
		pricesMap[region.Region] = make(map[string]string)
		for _, instanceType := range region.InstanceTypes {
			for _, size := range instanceType.Sizes {
				pricesMap[region.Region][size.Size] = size.ValueColumns[0].Prices.USD
			}
		}
	}
	return pricesMap
}

func getPrices() prices {
	b, err := ioutil.ReadFile("linux-od.min.js")
	if err != nil {
		log.Error(fmt.Sprintf("%s", err))
	}
	s := string(b)

	// strip initial comment (with newline)
	r := regexp.MustCompile("(?s)/\\*.*\\*/\n")
	s = r.ReplaceAllString(s, "")

	// strip from front of request
	r = regexp.MustCompile("^callback\\(")
	s = r.ReplaceAllString(s, "")

	// strip from end of request
	r = regexp.MustCompile("\\);*$")
	s = r.ReplaceAllString(s, "")

	var x prices
	// using json5 because I don't have to double quote keys
	// TODO: so hacky it hurts
	err = json5.Unmarshal([]byte(s), &x)
	if err != nil {
		log.Error(fmt.Sprintf("%s", err))
	}
	return x
}
