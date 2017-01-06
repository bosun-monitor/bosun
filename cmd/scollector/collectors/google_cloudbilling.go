package collectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/storage/v1"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

var gcbConf conf.GoogleCloudBilling
var gcbClient *storage.Service

const (
	gcbHoursInDay = 24
	gcbUsageDesc  = "Usage of consuming given Google Cloud services"
	gcbCostDesc   = "Cost of consuming given Google Cloud services"
)

func init() {
	registerInit(startGoogleCloudBilling)
}

// GoogleCloudBilling instantiates the GoogleCloudBilling collector
func startGoogleCloudBilling(c *conf.Conf) {
	for _, config := range c.GoogleCloudBilling {
		if config.BucketName == "" {
			slog.Errorln("No bucket name for Google Cloud Billing exports specified")
		}
		gcbConf = conf.GoogleCloudBilling{
			BucketName: config.BucketName,
			AuthBase64: config.AuthBase64,
		}
		newClient, err := createGCStorageClient()
		if err != nil {
			slog.Errorln("Could not create Google Cloud storage client: %s", err)
		}
		gcbClient = newClient
		collectors = append(collectors, &IntervalCollector{
			F:        c_googlecloudbilling,
			Interval: 1 * time.Hour,
		})
	}
}

func c_googlecloudbilling() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	//Get the list of available files from the storage bucket
	reports, err := gcbClient.Objects.List(gcbConf.BucketName).Do()
	if err != nil {
		return nil, err
	}
	//We want to process bills from the last 4 days. This is because Google Cloud seems to have significant
	//lag when generating the bill exports. They can be 2 or more days old by the time the bill is put into the
	//storage bucket.
	dateStart := time.Now().Add(-96 * time.Hour)
	dateRangeStart := time.Date(dateStart.Year(), dateStart.Month(), dateStart.Day(), 0, 0, 0, 0, time.UTC)
	dateRangeEnd := dateRangeStart.Add(120 * time.Hour).Add(-1 * time.Second)
	dateLayout := "2006-01-02"
	//Run through each item in the bucket
	for _, item := range reports.Items {
		//Is it a JSON file?
		fileNameParts := strings.Split(item.Name, ".")
		fileExt := strings.ToLower(fileNameParts[len(fileNameParts)-1])
		if fileExt != "json" {
			continue
		}
		//Check the date. We only want the _latest_ file. In case there are extra . in the filename,
		//the date will always be immediately before the extension
		fileName := fileNameParts[len(fileNameParts)-2]
		//If the last part of the filename does not have 10 or more characters, then it's not datestamped
		if len(fileName) < 10 {
			continue
		}
		dateString := fileName[len(fileName)-10:]
		dateDate, err := time.Parse(dateLayout, dateString)
		if err != nil {
			continue
		}
		if dateDate.Before(dateRangeStart) || dateDate.After(dateRangeEnd) {
			continue
		}
		//Ok, so we want to process this. Now we need to download the file, convert it to a string containing the JSON,
		//unmarshall that into the bill struct and send it off for processing
		thisItemResponse, err := gcbClient.Objects.Get(item.Bucket, item.Name).Download()
		if err != nil {
			return md, fmt.Errorf("gcloud: Could not download %s: %s", item.Name, err)
		}

		var thisBill gcBill
		decoder := json.NewDecoder(thisItemResponse.Body)
		err = decoder.Decode(&thisBill)
		if err != nil {
			return md, fmt.Errorf("gcloud: Could not unmarshal %s to JSON: %s", item.Name, err)
		}

		//Send it off for processing
		err = processGCBill(&md, &thisBill)
		if err != nil {
			return md, fmt.Errorf("gcloud: Could not process %s: %s", item.Name, err)
		}
	}
	return md, nil
}

// processGCBill will extract the info that it needs from the bill and append them to the tsdb data points
func processGCBill(md *opentsdb.MultiDataPoint, thisBill *gcBill) error {
	for _, item := range *thisBill {
		//This is the format that Google sends the date/time stamps in
		billTimeFormat := "2006-01-02T15:04:05-07:00"
		//We need to get actual date objects for the date/time strings. We want to send hourly data, but the duration
		//for the data in the bill is indeterminate - typically it's 24 hours, but it might not be. So we figure out
		//the duration here
		startTime, err := time.Parse(billTimeFormat, item.StartTime)
		if err != nil {
			slog.Errorf("gcloud: could not convert %s to time.Time: %s", item.StartTime, err)
			continue
		}
		endTime, err := time.Parse(billTimeFormat, item.EndTime)
		if err != nil {
			slog.Errorf("gcloud: could not convert %s to time.Time: %s", item.EndTime, err)
			continue
		}
		startTime = startTime.UTC() //We want to send UTC data
		endTime = endTime.UTC()

		//Now that we have proper start/end times, we can figure out how many hours there was inbetween,
		//and the values per hour. In the event of partial hours, we will round down.
		hoursInDuration := int(math.Floor(endTime.Sub(startTime).Hours()))
		costForItem, err := strconv.ParseFloat(item.Cost.Amount, 64)
		if err != nil {
			slog.Errorf("gcloud: could not convert %s to float64: %s", item.Cost.Amount, err)
			continue
		}
		costPerHour := costForItem / float64(hoursInDuration)
		//There isn't a huge amount of billing info, but there's all the useful things there. Description is added
		//for friendly display, but the Item tag should be used, as it will be consistent between revisions of the api
		//and also matches the measurement value
		costTags := opentsdb.TagSet{
			"projectid":   item.ProjectID,
			"description": item.Description,
			"item":        item.LineItemID,
		}
		//Add a datapoint for each hour in the duration
		for i := 0; i < hoursInDuration; i++ {
			recordTime := startTime.Add(time.Duration(i) * time.Hour)
			AddTS(md, "google.cloud.billing.cost", recordTime.Unix(), costPerHour, costTags, metadata.Gauge, metadata.Count, gcbCostDesc)
		}
		//There can be more than one measurement per item, so we need to add them all seperately
		for _, measurement := range item.Measurements {
			//Same deal as above, break the measurement down over the time period for the item
			usageForMeasurement, err := strconv.ParseFloat(measurement.Sum, 64)
			if err != nil {
				continue
			}
			usagePerHour := usageForMeasurement / float64(hoursInDuration)
			//Measurement does not provide a nice "description", so we will just use the item id
			usageTags := opentsdb.TagSet{
				"projectid": item.ProjectID,
				"item":      measurement.MeasurementID,
			}
			//And add a datapoint for each hour in the duration
			for i := 0; i < hoursInDuration; i++ {
				recordTime := startTime.Add(time.Duration(i) * time.Hour)
				AddTS(md, "google.cloud.billing.usage", recordTime.Unix(), usagePerHour, usageTags, metadata.Gauge, metadata.Count, gcbCostDesc)
			}
		}
	}
	return nil
}

// createGCStorageClient creates a storage client which uses the base64 encoded authtication JSON to create its connection.
// This returns a client that can be used for talking to the storage buckets directly.
func createGCStorageClient() (*storage.Service, error) {
	//We have our config in a big base64 string, so we need to get our JSON out of it
	authJSON, err := base64.StdEncoding.DecodeString(gcbConf.AuthBase64)
	if err != nil {
		return nil, err
	}
	//Generate a JWT using the JSON as constructed using the decoded JSON blob from above
	jwt, err := google.JWTConfigFromJSON(authJSON, storage.DevstorageReadOnlyScope)
	if err != nil {
		return nil, err
	}
	//Create a client and return it
	ctx := context.Background()
	hc := jwt.Client(ctx)
	return storage.New(hc)
}

type gcBill []struct {
	AccountID     string              `json:"accountId"`
	LineItemID    string              `json:"lineItemId"`
	Description   string              `json:"description"`
	StartTime     string              `json:"startTime"`
	EndTime       string              `json:"endTime"`
	ProjectNumber string              `json:"projectNumber"`
	ProjectID     string              `json:"projectId"`
	ProjectName   string              `json:"projectName"`
	Measurements  []gcBillMeasurement `json:"measurements"`
	Cost          gcBillCost          `json:"cost"`
}

type gcBillMeasurement struct {
	MeasurementID string `json:"measurementId"`
	Sum           string `json:"sum"`
	Unit          string `json:"unit"`
}

type gcBillCost struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
