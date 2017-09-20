package collectors

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhenderson-so/azure-ea-billing"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var azBillConf = azureEABillingConfig{}

const (
	hoursInDay = 24
	usageDesc  = "Usage of Azure service. Category is concatenated meter details. Resource is concatenated resource group and resource name."
	costDesc   = "Cost of Azure service. Category is concatenated meter details. Resource is concatenated resource group and resource name."
	priceDesc  = "Azure price sheet data for Enterprise Agreement services"
)

func init() {
	registerInit(startAzureEABilling)
}

func startAzureEABilling(c *conf.Conf) {
	for _, config := range c.AzureEA {
		if config.EANumber > 0 && config.APIKey != "" {
			azBillConf = azureEABillingConfig{
				AZEABillingConfig: azureeabilling.Config{
					EA:     config.EANumber,
					APIKey: config.APIKey,
				},
				CollectorConfig: config,
			}

			collectors = append(collectors, &IntervalCollector{
				F:        c_azureeabilling,
				Interval: 1 * time.Hour,
			})
		}
	}
}

func c_azureeabilling() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	//Get the list of available bills from the portal
	reports, err := azBillConf.AZEABillingConfig.GetUsageReports()
	if err != nil {
		return nil, err
	}

	//Process the report list
	if err = processAzureEAReports(reports, &md); err != nil {
		return nil, err
	}

	return md, nil

}

// processAzureEAReports will go through the monthly reports provided and pull out the ones that we're going to process
func processAzureEAReports(reports *azureeabilling.UsageReports, md *opentsdb.MultiDataPoint) error {
	baseTime := time.Now()
	thisMonth := baseTime.Format("2006-01")
	lastMonth := time.Date(baseTime.Year(), baseTime.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
	for _, r := range reports.AvailableMonths {
		//There's potentially a lot of reports. We only want to process this months + last months report
		if !(thisMonth == r.Month || lastMonth == r.Month) {
			continue
		}

		csv := azBillConf.AZEABillingConfig.GetMonthReportsCSV(r, azureeabilling.DownloadForStructs)
		structs, err := csv.ConvertToStructs()

		if err != nil {
			return err
		}
		for _, p := range structs.PriceSheetReport {
			err := processAzureEAPriceSheetRow(p, md)
			if err != nil {
				return err
			}
		}
		for _, d := range structs.DetailReport {
			err := processAzureEADetailRow(d, md)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// processAzureEAPriceSheetRow will take the price sheet info and log it, so we can track price changes over time
func processAzureEAPriceSheetRow(p *azureeabilling.PriceSheetRow, md *opentsdb.MultiDataPoint) error {
	fullProdName := fmt.Sprintf("%s-%s", p.Service, p.UnitOfMeasure)
	priceString := convertAzurePriceToString(p.UnitPrice)
	tags := opentsdb.TagSet{
		"partnumber": p.PartNumber,
		"service":    fullProdName,
	}
	Add(md, "azure.ea.pricesheet", priceString, tags, metadata.Gauge, metadata.Count, priceDesc)
	return nil
}

// processAzureEADetailRow will take the actual usage data for the provided month
func processAzureEADetailRow(p *azureeabilling.DetailRow, md *opentsdb.MultiDataPoint) error {
	//Don't process todays records as they are subject to change
	nowYear, nowMonth, nowDay := time.Now().Date()
	recordMonth := int(nowMonth)
	if nowYear == p.Year && recordMonth == p.Month && nowDay == p.Day {
		return nil
	}

	resourcePaths := strings.Split(strings.ToLower(p.InstanceID), "/")
	var resourceString string

	if len(resourcePaths) < 8 {
		resourceString = strings.ToLower(p.InstanceID)
	} else {
		resourceIDs := resourcePaths[8:]
		resourceString = strings.Join(resourceIDs, "-")
	}

	if p.ResourceGroup != "" {
		resourceString = fmt.Sprintf("%s-%s", strings.ToLower(p.ResourceGroup), resourceString)
	}

	tags := opentsdb.TagSet{
		"category":    p.MeterCategory,
		"subcategory": fmt.Sprintf("%s-%s", strings.ToLower(p.MeterSubCategory), strings.ToLower(p.MeterName)),
	}

	resourceString, err := opentsdb.Clean(resourceString)
	if err != nil && resourceString != "" {
		tags["resource"] = resourceString
	}

	//Only log resource group details if they are enabled in the config
	if azBillConf.CollectorConfig.LogResourceDetails {
		resourcLocation, _ := opentsdb.Clean(p.ResourceLocation)
		resouceGroup, _ := opentsdb.Clean(p.ResourceGroup)
		if resouceGroup != "" {
			tags["resoucegroup"] = strings.ToLower(resouceGroup)
		}
		if resourcLocation != "" {
			tags["resourcelocation"] = strings.ToLower(resourcLocation)
		}
	}

	//Only log extra Azure tags if enabled in the config
	if azBillConf.CollectorConfig.LogExtraTags {
		if p.Tags != "" {
			customTags := make(map[string]string)
			json.Unmarshal([]byte(p.Tags), &customTags)
			for t, v := range customTags {
				if t[:6] == "hidden" {
					continue
				}
				value, _ := opentsdb.Clean(v)
				if value == "" {
					continue
				}
				tags[strings.ToLower(t)] = strings.ToLower(value)
			}
		}
	}

	//Only log billing details if they are enabled in the config
	if azBillConf.CollectorConfig.LogBillingDetails {
		if p.CostCenter != "" {
			tags["costcenter"] = strings.ToLower(p.CostCenter)
		}
		cleanAccountName, _ := opentsdb.Clean(p.AccountName)
		tags["accountname"] = strings.ToLower(cleanAccountName)
		tags["subscription"] = strings.ToLower(p.SubscriptionName)
	}

	recordDate := time.Date(p.Year, time.Month(p.Month), p.Day, 0, 0, 0, 0, time.UTC)

	//Because we need to log this hourly and we only have daily data, divide the daily cost into hourly costs
	qtyPerHour := p.ConsumedQuantity / hoursInDay

	//ExtendedCost is stored only in a string, because it's a variable number of decimal places. Which means we can't reliably store it in an int, and storing in a float reduces precision.
	//This way we're choosing ourselves to drop the precision, which adds up to around 10-20c under initial testing.
	costPerDay, err := strconv.ParseFloat(p.ExtendedCostRaw, 64)
	if err != nil {
		return err
	}
	costPerHour := costPerDay / hoursInDay

	//Get 24 records for 24 hours in a day
	for i := 0; i < hoursInDay; i++ {
		recordTime := recordDate.Add(time.Duration(i) * time.Hour)
		AddTS(md, "azure.ea.usage", recordTime.Unix(), qtyPerHour, tags, metadata.Gauge, metadata.Count, usageDesc)
		AddTS(md, "azure.ea.cost", recordTime.Unix(), costPerHour, tags, metadata.Gauge, metadata.Count, costDesc)
	}

	return nil
}

//The cost is stored in cents, and we want to translate the cent cost into dollars and cents, but in a string
//which will not lose precision and is close enough for government work.
func convertAzurePriceToString(costInCents int) string {
	priceString := strconv.Itoa(costInCents)
	priceLen := len(priceString)
	if priceLen == 1 {
		priceString = fmt.Sprintf("0.0%s", priceString)
	}
	if priceLen == 2 {
		priceString = fmt.Sprintf("0.%s", priceString)
	}
	if priceLen >= 3 {
		priceString = fmt.Sprintf("%s.%s", priceString[0:priceLen-2], priceString[priceLen-2:])
	}

	return priceString
}

type azureEABillingConfig struct {
	CollectorConfig   conf.AzureEA
	AZEABillingConfig azureeabilling.Config
}
