package azureeabilling

import (
	"net/http"
	"time"
)

//Config is the basic config struct for Azure EA billing
type Config struct {
	BaseURI    string
	EA         uint32
	APIKey     string
	httpClient *http.Client
}

// UsageReports is the struct containing the list of reports avaliable
type UsageReports struct {
	AvailableMonths []AvailableMonth `json:"AvailableMonths"`
	ContractVersion string           `json:"ContractVersion"`
	ObjectType      string           `json:"ObjectType"`
}

// AvailableMonth is used in UsageReports for the details for a given month
type AvailableMonth struct {
	LinkToDownloadDetailReport      string `json:"LinkToDownloadDetailReport"`
	LinkToDownloadPriceSheetReport  string `json:"LinkToDownloadPriceSheetReport"`
	LinkToDownloadStoreChargeReport string `json:"LinkToDownloadStoreChargeReport"`
	LinkToDownloadSummaryReport     string `json:"LinkToDownloadSummaryReport"`
	Month                           string `json:"Month"`
}

// MonthDownloadCSV is the struct for downloaded reports in CSV format
type MonthDownloadCSV struct {
	DetailReport      string
	PriceSheetReport  string
	StoreChargeReport string
	SummaryReport     string
}

// MonthDownloadStructs is the struct that contains other reports represented as structs
type MonthDownloadStructs struct {
	DetailReport     []*DetailRow
	PriceSheetReport []*PriceSheetRow
}

// PriceSheetRow contains the data of each row in a price sheet table
type PriceSheetRow struct {
	Service                string `csv:"Service"`
	UnitOfMeasure          string `csv:"Unit of Measure"`
	PartNumber             string `csv:"Part Number"`
	UnitPriceRaw           string `csv:"Unit Price"`
	UnitPrice              int    `csv:"-"`
	CommitmentPartNumber   string `csv:"Commitment Part Number"`
	CommitmentUnitPriceRaw string `csv:"Commitment Unit Price"`
	CommitmentUnitPrice    int    `csv:"-"`
	OveragePartNumber      string `csv:"Overage Part Number"`
	OverageUnitPriceRaw    string `csv:"Overage Unit Price"`
	OverageUnitPrice       int    `csv:"-"`

	CurrencyCode string `csv:"Currency Code"`
}

// DetailRow contains the data of each row in the monthly account detail usage table
type DetailRow struct {
	AccountOwnerID         string    `csv:"AccountOwnerId"`
	AccountName            string    `csv:"Account Name"`
	ServiceAdministratorID string    `csv:"ServiceAdministratorId"`
	SubscriptionID         string    `csv:"SubscriptionId"`
	SubscriptionGUID       string    `csv:"SubscriptionGuid"`
	SubscriptionName       string    `csv:"Subscription Name"`
	DateRaw                string    `csv:"Date"`
	Date                   time.Time `csv:"-"`
	Month                  int       `csv:"Month"`
	Day                    int       `csv:"Day"`
	Year                   int       `csv:"Year"`
	Product                string    `csv:"Product"`
	MeterID                string    `csv:"Meter ID"`
	MeterCategory          string    `csv:"Meter Category"`
	MeterSubCategory       string    `csv:"Meter Sub-Category"`
	MeterRegion            string    `csv:"Meter Region"`
	MeterName              string    `csv:"Meter Name"`
	ConsumedQuantity       float64   `csv:"Consumed Quantity"`
	ResourceRate           float64   `csv:"ResourceRate"`
	ExtendedCostRaw        string    `csv:"ExtendedCost"`
	ResourceLocation       string    `csv:"Resource Location"`
	ConsumedService        string    `csv:"Consumed Service"`
	InstanceID             string    `csv:"Instance ID"`
	ServiceInfo1           string    `csv:"ServiceInfo1"`
	ServiceInfo2           string    `csv:"ServiceInfo2"`
	AdditionalInfo         string    `csv:"AdditionalInfo"`
	Tags                   string    `csv:"Tags"`
	StoreServiceIdentifier string    `csv:"Store Service Identifier"`
	DepartmentName         string    `csv:"Department Name"`
	CostCenter             string    `csv:"Cost Center"`
	UnitOfMeasure          string    `csv:"Unit Of Measure"`
	ResourceGroup          string    `csv:"Resource Group"`
}

// DownloadOption is used as our enum to toggle which reports we do and don't want to download
type DownloadOption int

const (
	// DownloadDetail - Download the detail report
	DownloadDetail DownloadOption = 1
	// DownloadPricesheet - Download the price sheet
	DownloadPricesheet DownloadOption = 2
	// DownloadStoreCharge - Download the store charge
	DownloadStoreCharge DownloadOption = 4
	// DownloadSummary - Download the summary
	DownloadSummary DownloadOption = 8
	// DownloadAll - Download all of the reports
	DownloadAll DownloadOption = DownloadDetail | DownloadPricesheet | DownloadStoreCharge | DownloadSummary
	// Download the reports that we have struct support for
	DownloadForStructs DownloadOption = DownloadDetail | DownloadPricesheet
)
