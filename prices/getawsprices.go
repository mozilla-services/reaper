package prices

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/antonholmquist/jason"
	log "github.com/mozilla-services/reaper/reaperlog"
)

const Ec2PricingUrl = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json"

type PricesMap map[string]map[string]string

var regions = map[string]string{
	"US West (N. California)":   "us-west-1",
	"US West (Oregon)":          "us-west-2",
	"EU (Ireland)":              "eu-west-1",
	"EU (Frankfurt)":            "eu-central-1",
	"Asia Pacific (Seoul)":      "ap-northeast-2",
	"Asia Pacific (Sydney)":     "ap-southeast-2",
	"US East (N. Virginia)":     "us-east-1",
	"South America (Sao Paulo)": "sa-east-1",
	"Asia Pacific (Singapore)":  "ap-southeast-1",
	"Asia Pacific (Tokyo)":      "ap-northeast-1",
	"AWS GovCloud (US)":         "AWS GovCloud (US)",
}

func GetPricesMapFromFile(filename string) (PricesMap, error) {
	if filename == "" {
		return PricesMap{}, nil
	}
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return populatePricesMap(bytes.NewReader(bs))
}

func DownloadPricesMap(url string) (PricesMap, error) {
	if url == "" {
		return PricesMap{}, nil
	}

	// TODO: working TLS
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	res, err := client.Get(url)
	if err != nil {
		log.Error(err.Error())
	}
	defer res.Body.Close()
	return populatePricesMap(res.Body)
}

func populatePricesMap(r io.Reader) (PricesMap, error) {
	pricesMap := make(PricesMap)
	for _, region := range regions {
		pricesMap[region] = make(map[string]string)
	}

	// initialize inner map
	decoded, err := jason.NewObjectFromReader(r)
	if err != nil {
		return PricesMap{}, err
	}
	products, err := decoded.GetObject("products")
	if err != nil {
		return PricesMap{}, err
	}
	for _, skuObject := range products.Map() {
		individualSkuObject, err := skuObject.Object()
		if err != nil {
			return PricesMap{}, err
		}
		sku, err := individualSkuObject.Map()["sku"].String()
		if err != nil {
			return PricesMap{}, err
		}
		attributes, err := individualSkuObject.Map()["attributes"].Object()
		if err != nil {
			return PricesMap{}, err
		}
		if instanceType, ok := attributes.Map()["instanceType"]; !ok {
			// if it's not an instance, skip it
			continue
		} else {
			instanceTypeString, err := instanceType.String()
			if err != nil {
				return PricesMap{}, err
			}
			location, err := attributes.Map()["location"].String()
			if err != nil {
				return PricesMap{}, err
			}
			// get pricing information from a separate part of the json file by sku
			priceSkuObject, err := decoded.GetObject("terms", "OnDemand", sku)
			if err != nil {
				return PricesMap{}, err
			}
			for _, specificSku := range priceSkuObject.Map() {
				specificSkuObject, err := specificSku.Object()
				if err != nil {
					return PricesMap{}, err
				}
				priceDimensionsObject, err := specificSkuObject.Map()["priceDimensions"].Object()
				if err != nil {
					return PricesMap{}, err
				}
				for _, specificPriceDimensions := range priceDimensionsObject.Map() {
					specificPriceDimensionsObject, err := specificPriceDimensions.Object()
					if err != nil {
						return PricesMap{}, err
					}
					pricePerUnitObject, err := specificPriceDimensionsObject.Map()["pricePerUnit"].Object()
					if err != nil {
						return PricesMap{}, err
					}
					USD, err := pricePerUnitObject.Map()["USD"].String()
					if err != nil {
						return PricesMap{}, err
					}
					pricesMap[regions[location]][instanceTypeString] = USD
				}
			}
		}
	}
	return pricesMap, nil
}
