package prices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

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
	"US East (Ohio)":            "us-east-2",
	"South America (Sao Paulo)": "sa-east-1",
	"Asia Pacific (Singapore)":  "ap-southeast-1",
	"Asia Pacific (Tokyo)":      "ap-northeast-1",
	"Asia Pacific (Mumbai)":     "ap-south-1",
	"AWS GovCloud (US)":         "AWS GovCloud (US)",
}

type ProductPriceData struct {
	Attributes struct {
		ClockSpeed            string `json:"clockSpeed"`
		CurrentGeneration     string `json:"currentGeneration"`
		InstanceFamily        string `json:"instanceFamily"`
		InstanceType          string `json:"instanceType"`
		LicenseModel          string `json:"licenseModel"`
		Location              string `json:"location"`
		LocationType          string `json:"locationType"`
		Memory                string `json:"memory"`
		NetworkPerformance    string `json:"networkPerformance"`
		OperatingSystem       string `json:"operatingSystem"`
		Operation             string `json:"operation"`
		PhysicalProcessor     string `json:"physicalProcessor"`
		PreInstalledSw        string `json:"preInstalledSw"`
		ProcessorArchitecture string `json:"processorArchitecture"`
		Servicecode           string `json:"servicecode"`
		Storage               string `json:"storage"`
		Tenancy               string `json:"tenancy"`
		Usagetype             string `json:"usagetype"`
		Vcpu                  string `json:"vcpu"`
	} `json:"attributes"`
	ProductFamily string `json:"productFamily"`
	Sku           string `json:"sku"`
}

type TermData struct {
	EffectiveDate   string `json:"effectiveDate"`
	OfferTermCode   string `json:"offerTermCode"`
	PriceDimensions map[string]struct {
		BeginRange   string `json:"beginRange"`
		Description  string `json:"description"`
		EndRange     string `json:"endRange"`
		PricePerUnit struct {
			USD string `json:"USD"`
		} `json:"pricePerUnit"`
		RateCode string `json:"rateCode"`
		Unit     string `json:"unit"`
	} `json:"priceDimensions"`
	Sku            string   `json:"sku"`
	TermAttributes struct{} `json:"termAttributes"`
}

type PriceData struct {
	Disclaimer    string                      `json:"disclaimer"`
	FormatVersion string                      `json:"formatVersion"`
	OfferCode     string                      `json:"offerCode"`
	Products      map[string]ProductPriceData `json:"products"`
	Terms         struct {
		OnDemand map[string]map[string]TermData
		Reserved map[string]map[string]TermData
	}
	PublicationDate string `json:"publicationDate"`
	Version         string `json:"version"`
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
		return PricesMap{}, fmt.Errorf("Invalid price url")
	}

	res, err := http.Get(url)
	if err != nil {
		return PricesMap{}, err
	}
	defer res.Body.Close()
	return populatePricesMap(res.Body)
}

func populatePricesMap(r io.Reader) (PricesMap, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from a panic: ", r)
		}
	}()

	// initialize inner map
	pricesMap := make(PricesMap)
	for _, region := range regions {
		pricesMap[region] = make(map[string]string)
	}

	pd := new(PriceData)
	err := json.NewDecoder(r).Decode(pd)
	if err != nil {
		return PricesMap{}, err
	}

	for sku, productData := range pd.Products {
		// only get prices for EC2 instances
		if productData.ProductFamily != "Compute Instance" {
			continue
		}
		for _, termData := range pd.Terms.OnDemand[sku] {
			for _, dimensionData := range termData.PriceDimensions {
				if region, ok := regions[productData.Attributes.Location]; ok {
					pricesMap[region][productData.Attributes.InstanceType] = dimensionData.PricePerUnit.USD
				} else {
					log.Error(fmt.Sprintf("Region not found for sku %s location %s", sku, productData.Attributes.Location))
				}
			}
		}
	}

	return pricesMap, nil
}
