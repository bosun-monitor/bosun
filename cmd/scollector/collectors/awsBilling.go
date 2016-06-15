package collectors

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	awsBillingR53zones = map[string]route53.GetHostedZoneOutput{}
)

const (
	awsBillingReportDateFormat = "20060102"
)

func c_awsBilling(accessKey, secretKey, region, productCodes, bucketName, bucketPath string, purgeDays int) (opentsdb.MultiDataPoint, error) {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	conf := &aws.Config{
		Credentials: creds,
		Region:      &region,
	}
	awsBilling := awsBillingConfig{
		bucketName: bucketName,
		bucketPath: bucketPath,
	}
	regCompiled, err := regexp.Compile(productCodes)
	if err != nil {
		return nil, err
	}
	awsBilling.prodCodesReg = regCompiled
	awsBilling.s3svc = s3.New(session.New(conf)) //Connect to S3
	if awsBilling.s3svc == nil {
		return nil, fmt.Errorf("unable to connect to S3")
	}
	awsBilling.r53svc = route53.New(session.New(conf)) //Connect to R53
	if awsBilling.r53svc == nil {
		return nil, fmt.Errorf("unable to connect to Route 53")
	}
	awsBilling.downloader = s3manager.NewDownloader(session.New(conf)) //Gimmie a downloader
	if awsBilling.downloader == nil {
		return nil, fmt.Errorf("unable to create S3 downloader")
	}
	if purgeDays == 0 {
		slog.Infof("S3 purging of objects is disabled")
		awsBilling.purgeOlderThan = time.Date(2999, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		purgeHours := time.Duration(-1 * 24 * purgeDays)
		awsBilling.purgeOlderThan = time.Now().Add(purgeHours * time.Hour)
	}
	return awsBilling.Go()
}

type awsBillingConfig struct {
	s3svc          *s3.S3
	r53svc         *route53.Route53
	downloader     *s3manager.Downloader
	purgeOlderThan time.Time
	bucketName     string
	bucketPath     string
	prodCodesReg   *regexp.Regexp
}

func (awsBilling *awsBillingConfig) Go() (opentsdb.MultiDataPoint, error) {
	md := opentsdb.MultiDataPoint{}
	purgeObjects := []*s3.Object{}
	//Declare the objects we want to fetch here. For completeness sake, we're going to fetch
	//the entire contents of the bucket, as we are going to be cleaning it out as we go.
	getBucketObjects := &s3.ListObjectsInput{
		Bucket: aws.String(awsBilling.bucketName),
	}
	//Get the objects from the bucket
	bucketObjects, err := awsBilling.s3svc.ListObjects(getBucketObjects)
	if err != nil {
		return nil, err
	}
	//Go through the contents of the bucket and parse the different kinds of report files
	//that that we're going to have. At the end of it we'll have a bucketContents that contains
	//the origina S3 file, along with hopefully enough other detail.
	var thisBucketContents []bucketContents
	for _, bucketObject := range bucketObjects.Contents {
		thisReport := &billingKeyStructure{}
		thisReport.parseFromObjectKey(bucketObject.Key, awsBilling)

		thisBucketContents = append(thisBucketContents, bucketContents{
			origS3Item:     bucketObject,
			awsBillingItem: thisReport,
		})

		if bucketObject.LastModified.Before(awsBilling.purgeOlderThan) {
			purgeObjects = append(purgeObjects, bucketObject)
		}
	}
	//So, billing comes with a bunch of different manifets and whatnot. We don't really need all of that
	//for this basic billing integration, as the first line of the CSVs we're interested in also has
	//the breakdown of how it's all structured.
	var allBills []*billBill
	for _, billingObject := range thisBucketContents {
		//If the filename ends in a .gz then it's probably what we want
		if filepath.Ext(billingObject.awsBillingItem.fileName) == ".gz" {
			//if it ends in .gz and has the report name in it, then it's definately what we want
			if strings.Contains(billingObject.awsBillingItem.fileName, billingObject.awsBillingItem.reportName) {
				//Download the file and un-gzip it in one step
				billingFile, err := billingObject.downloadGzippedItem(awsBilling.downloader, awsBilling)
				if err != nil {
					return nil, err
				}
				allBills = append(allBills, awsBilling.ReadBillingFile(billingFile))
			}
		}
	}
	//Dump into scollector format and clean up
	for _, thisBill := range allBills {
		thisBill.toTSDB(&md)
	}
	//Clean up after ourselves
	for _, purge := range purgeObjects {
		slog.Infoln("Need to purge object", *purge.Key, "last modified", purge.LastModified)
		_, err := awsBilling.s3svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(awsBilling.bucketName),
			Key:    purge.Key,
		})
		if err != nil {
			slog.Warningf("Error deleting object:", err)
		}
	}
	return md, nil
}

type billBill struct {
	InvoiceID              string
	BillingEntity          string
	BillType               string
	PayerAccountID         string
	BillingPeriodStartDate time.Time
	BillingPeriodEndDate   time.Time
	LineItems              []billLineItem
}

func (b billBill) toTSDB(md *opentsdb.MultiDataPoint) {
	productCodes := []string{}
	for _, line := range b.LineItems {
		tags := make(opentsdb.TagSet)
		//Keep track of the unique product codes we've seen so we can
		//send metadata for them later
		if !stringInSlice(line.ProductCode, productCodes) {
			productCodes = append(productCodes, line.ProductCode)
		}
		//Add the host tag, and delete the host tag if it's blank
		if host, present := tags["host"]; !present {
			tags["host"] = util.Hostname
		} else if host == "" {
			delete(tags, "host")
		}
		//Suppressing AccountID for now as we only have one account
		//tags["accountid"] = line.UsageAccountID
		tags["operation"] = line.Operation
		metricCode := strings.ToLower(line.ProductCode)
		metricCode = strings.Replace(metricCode, "amazon", "", 1)
		//Usage Type is meaningless for R53, they're all 'DNS-Queries'
		if line.ProductCode != "AmazonRoute53" {
			tags["usagetype"] = line.UsageType
		}
		//Use the zone name if present for Route 53 data
		if line.ProductCode == "AmazonRoute53" && line.Route53Zone != nil && *line.Route53Zone.Name != "" {
			tags["resourceid"] = strings.ToLower(*line.Route53Zone.Name)
		} else if line.ResourceID != "" {
			//Remove pesky invalid characters that show up. Probably should redo this as a regex
			tags["resourceid"] = strings.ToLower(strings.Replace(strings.Replace(strings.Replace(strings.Replace(line.ResourceID, "/", "-", -1), ":", "-", -1), "[", "", -1), "]", "", -1))
		}
		//For our denormalized metrics, just keep the host tag, and the required tag
		//Some entries do not have a resource ID (e.g. access denied errors, or bucket does not exist, for obvious reasons)
		tagsByResource := opentsdb.TagSet{"host": tags["host"]}
		if resid, present := tags["resourceid"]; present {
			tagsByResource["resourceid"] = resid
		}
		tagsByOperation := opentsdb.TagSet{"host": tags["host"], "operation": tags["operation"]}
		descCosts := fmt.Sprintf("Usage costs for Amazon %v. Datapoints represent costs for a full hour. Data typically lags by 24 hours.", metricCode)
		descUsage := fmt.Sprintf("Usage volume for Amazon %v. Datapoints represent a full hour of usage. Data typically lags by 24 hours.", metricCode)
		descUsageRes := fmt.Sprintf("Usage volume for Amazon %v, denormalized with only a resource tag. Datapoints represent a full hour of usage. Data typically lags by 24 hours.", metricCode)
		descUsageOps := fmt.Sprintf("Usage volume for Amazon %v, denormalized with only an operation tag. Datapoints represent a full hour of usage. Data typically lags by 24 hours.", metricCode)
		//Glacier bills by day, not by hour, for storage, so we need to fill in some fake datapoints
		if line.ProductCode == "AmazonGlacier" && line.Operation == "Storage" {
			for i := 0; i < 24; i++ {
				AddTS(md, fmt.Sprintf("aws.billing.%v.cost", metricCode), line.UsageEndDate.Add(time.Duration(-1*i)*time.Hour).Unix(), line.UnblendedCost/24, tags, metadata.Gauge, metadata.Count, descCosts)
				AddTS(md, fmt.Sprintf("aws.billing.%v.usage", metricCode), line.UsageEndDate.Add(time.Duration(-1*i)*time.Hour).Unix(), line.UsageAmount, tags, metadata.Gauge, metadata.Count, descUsage)
				AddTS(md, fmt.Sprintf("aws.billing.%v.usage_by_operation", metricCode), line.UsageEndDate.Add(time.Duration(-1*i)*time.Hour).Unix(), line.UsageAmount, tagsByOperation, metadata.Gauge, metadata.Count, descUsageOps)
				if _, present := tagsByResource["resourceid"]; present {
					AddTS(md, fmt.Sprintf("aws.billing.%v.usage_by_resource", metricCode), line.UsageEndDate.Add(time.Duration(-1*i)*time.Hour).Unix(), line.UsageAmount, tagsByResource, metadata.Gauge, metadata.Count, descUsageRes)
				}
			}
		} else {
			AddTS(md, fmt.Sprintf("aws.billing.%v.cost", metricCode), line.UsageEndDate.Unix(), line.UnblendedCost, tags, metadata.Gauge, metadata.Count, descCosts)
			AddTS(md, fmt.Sprintf("aws.billing.%v.usage", metricCode), line.UsageEndDate.Unix(), line.UsageAmount, tags, metadata.Gauge, metadata.Count, descUsage)
			AddTS(md, fmt.Sprintf("aws.billing.%v.usage_by_operation", metricCode), line.UsageEndDate.Unix(), line.UsageAmount, tagsByOperation, metadata.Gauge, metadata.Count, descUsageOps)
			if _, present := tagsByResource["resourceid"]; present {
				AddTS(md, fmt.Sprintf("aws.billing.%v.usage_by_resource", metricCode), line.UsageEndDate.Unix(), line.UsageAmount, tagsByResource, metadata.Gauge, metadata.Count, descUsageRes)

			}
		}
	}
}

type billLineItem struct {
	IdentityLineItemID  string `csv:"identity/LineItemId"`
	UsageAccountID      string `csv:"lineItem/UsageAccountId"`
	LineItemType        string `csv:"lineItem/LineItemType"`
	UsageStartDateRaw   string `csv:"lineItem/UsageStartDate"`
	UsageEndDateRaw     string `csv:"lineItem/UsageEndDate"`
	UsageStartDate      time.Time
	UsageEndDate        time.Time
	ProductCode         string  `csv:"lineItem/ProductCode"`
	UsageType           string  `csv:"lineItem/UsageType"`
	Operation           string  `csv:"lineItem/Operation"`
	AvailabilityZone    string  `csv:"lineItem/AvailabilityZone"`
	ResourceID          string  `csv:"lineItem/ResourceId"`
	UsageAmount         float32 `csv:"lineItem/UsageAmount"`
	CurrencyCode        string  `csv:"lineItem/CurrencyCode"`
	UnblendedRate       float32 `csv:"lineItem/UnblendedRate"`
	UnblendedCost       float32 `csv:"lineItem/UnblendedCost"`
	BlendedRate         float32 `csv:"lineItem/BlendedRate"`
	BlendedCost         float32 `csv:"lineItem/BlendedCost"`
	LineItemDescription string  `csv:"lineItem/LineItemDescription"`
	TaxType             string  `csv:"lineItem/TaxType"`
	Route53Zone         *route53.HostedZone
}

//Using reflection over the billLineItem struct, construct a row based on the
//data from the CSV
func (b *billLineItem) parseFromBill(row []string, cols map[string]int, awsBilling *awsBillingConfig) {
	val := reflect.ValueOf(b).Elem()      //Reflect the elements of the billLineItem struct
	for i := 0; i < val.NumField(); i++ { //Run through the number of fields on the struct
		valueField := val.Field(i)                            //Get the reflect value for this item on the struct
		typeField := val.Type().Field(i)                      //Get the type of this value
		csvField := strings.ToUpper(typeField.Tag.Get("csv")) //Get the corresponding CSV tag from the type
		if valueField.CanSet() {                              //If this is a settable value
			switch valueField.Kind() { //We only deal with strings and float32's here, as the invoice is pretty simple
			case reflect.String: //If we have a string
				valueField.SetString(row[cols[csvField]]) //Set the string to the value from the column labelled by the CSV tag
			case reflect.Float32: //If we have a float32
				float, err := strconv.ParseFloat(row[cols[csvField]], 32) //Convert to a 32-bit float
				if err == nil {
					valueField.SetFloat(float) //Set the string to the value from the column labelled by the CSV tag
				}
			}
		}
	}
	//Fill in the date/time fields from their raw counterparts
	b.UsageStartDate, _ = time.Parse(time.RFC3339, b.UsageStartDateRaw)
	b.UsageEndDate, _ = time.Parse(time.RFC3339, b.UsageEndDateRaw)
	//Fetch the Route53 data for this row
	b.fetchR53(awsBilling)
}

//If there is Route53 data for this row, then populate the Route 53 item
func (b *billLineItem) fetchR53(awsBilling *awsBillingConfig) {
	if b.ProductCode == "AmazonRoute53" { //Don't do anything if we don't have any R53 info to get
		zoneID := strings.Split(b.ResourceID, "/")[1]   //The billing ID has a huge resource ID, we only need the last part of it
		cachedR53Zone, ok := awsBillingR53zones[zoneID] //Check if we have a copy of this zone in our local cache
		if ok {                                         //If we have a copy of the zone, then use that
			b.Route53Zone = cachedR53Zone.HostedZone
		} else { //Otherwise we need to fetch it from Route 53
			thisR53, fetchErr := awsBilling.r53svc.GetHostedZone(&route53.GetHostedZoneInput{
				Id: aws.String(zoneID),
			})
			if fetchErr != nil {
				slog.Infoln("Cannot fetch Route53 hosted zone", b.ResourceID, fetchErr)
			}
			awsBillingR53zones[zoneID] = *thisR53 //Store the fetched zone in the cache
			b.Route53Zone = thisR53.HostedZone    //And assign
		}
	}
}

type bucketContents struct {
	origS3Item     *s3.Object
	awsBillingItem *billingKeyStructure
}

//Download a GZipped item from S3 and return the non-gzipped version of the item
func (billingObject *bucketContents) downloadGzippedItem(downloader *s3manager.Downloader, awsBilling *awsBillingConfig) (*[]byte, error) {
	//Get a temporary file to dump this into
	tempFile, err := ioutil.TempFile("", "scollector-aws-billing-")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	_, err = downloader.Download(tempFile,
		&s3.GetObjectInput{
			Bucket: aws.String(awsBilling.bucketName),
			Key:    billingObject.origS3Item.Key,
		})
	if err != nil {
		return nil, err
	}
	unzippedFile, err := readGzFile(tempFile)
	if err != nil {
		return nil, err
	}
	tempFile.Close()
	err = os.Remove(tempFile.Name())
	if err != nil {
		slog.Warningf("Could not remove temporary file", tempFile.Name())
	}
	return &unzippedFile, err
}

//http://stackoverflow.com/a/23635064/69683
func readGzFile(fi *os.File) ([]byte, error) {
	fz, err := gzip.NewReader(fi)
	if err != nil {
		return nil, err
	}
	defer fz.Close()

	s, err := ioutil.ReadAll(fz)
	if err != nil {
		return nil, err
	}
	return s, nil
}

type billingKeyStructure struct {
	reportName  string
	reportStart time.Time
	reportEnd   time.Time
	reportID    string
	fileName    string
	filePath    string
}

//This takes the S3 key for the items (which is the file path) and uses that to populate
//a billing key structure, which contains the dates, times, type of report, etc
func (b *billingKeyStructure) parseFromObjectKey(key *string, awsBilling *awsBillingConfig) {
	keyDir, keyFile := filepath.Split(*key)
	if keyDir == "" || keyFile == "" {
		return
	}
	dirParts := strings.Split(keyDir, "/")
	if dirParts[0] != awsBilling.bucketPath {
		return
	}
	dateParts := strings.Split(dirParts[2], "-")
	b.reportName = dirParts[1]
	b.reportStart, _ = time.Parse(awsBillingReportDateFormat, dateParts[0])
	b.reportEnd, _ = time.Parse(awsBillingReportDateFormat, dateParts[1])
	if len(dirParts) == 5 {
		b.reportID = dirParts[3]
	}
	b.fileName = keyFile
	b.filePath = *key
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//Process the billing file and spit out a bill that contains the bill header and
//line items
func (awsBilling *awsBillingConfig) ReadBillingFile(contents *[]byte) *billBill {
	//Parse the bill's CSV format
	r := csv.NewReader(strings.NewReader(string(*contents)))
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	//This is going to contain our mapping of fields to columns. First line of the
	//CSV is the field names. If they change in the future (added/deleted/re-org'd)
	//then we don't need to screw around with static mappings
	cols := make(map[string]int)
	//Get the columns from the first row, which are the field names, and throw
	//them into our struct with their correct position.
	for colNo, headerVal := range records[0] {
		cols[strings.ToUpper(headerVal)] = colNo
	}
	//Somewhere to store our bill as we parse it
	var thisBill billBill
	//Work through the invoice rows from 1 till the end
	for rowNo, row := range records[1:] {
		if rowNo == 1 { //This general invoice stuff is the same on every row, so we only need to parse it once
			thisBill.InvoiceID = val(row, cols, "bill/InvoiceId")
			thisBill.BillingEntity = val(row, cols, "bill/BillingEntity")
			thisBill.BillType = val(row, cols, "bill/BillType")
			thisBill.PayerAccountID = val(row, cols, "bill/PayerAccountId")
			thisBill.BillingPeriodStartDate, _ = time.Parse(time.RFC3339, val(row, cols, "bill/BillingPeriodStartDate"))
			thisBill.BillingPeriodEndDate, _ = time.Parse(time.RFC3339, val(row, cols, "bill/BillingPeriodEndDate"))
		}
		//Only process lines that have a product code we care about
		if awsBilling.prodCodesReg.MatchString(val(row, cols, "lineItem/ProductCode")) {
			//Parse the contents of the row
			thisBillLine := &billLineItem{}                                //Somewhere to store our row
			thisBillLine.parseFromBill(row, cols, awsBilling)              //Parse it
			thisBill.LineItems = append(thisBill.LineItems, *thisBillLine) //Add it to the bill
		}
	}
	return &thisBill
}

//Return the a given column's value from a row, given a field
func val(row []string, cols map[string]int, field string) string {
	return row[cols[strings.ToUpper(field)]]
}
