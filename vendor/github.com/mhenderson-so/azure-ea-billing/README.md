# azure-ea-billing
A small GoLang package for retreiving billing data from the Azure Enterprise Agreement billing portal.

This is *not* a package that can retrieve billing information from traditional Azure subscriptions. This is _only_
for enterprise agreements, as it talks to the EA billing portal (not the usual Azure billing portal.)

# Usage
```
import "github.com/mhenderson-so/azure-ea-billing"

func main() {
	eab := azureeabilling.Config{
		EA:     12345678,  //Your EA Number
		APIKey: "abcdefg", //Your EA Billing API Key
	}

	resp, err := eab.GetUsageReports() //Fetches the usage reports from the EA portal
	if err != nil {
		fmt.Println(err)
		return
	}
    
	//Fetches a specific month from the EA billing portal
	reports := eab.GetMonthReportsCSV(resp.AvailableMonths[20], azureeabilling.DownloadForStructs)

	//Convert the CSV reports to GoLang structs so we can access the data easier
	structs, err := reports.ConvertToStructs()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, l := range structs.DetailReport {
		fmt.Println(*l)
	}
}
```

# Supported reports
Any of the EA portal reports can be downloaded in CSV format (depending on what is avaliable in your EA portal).

The following two reports can be converted to structs:

- Detail report
- Price Sheet
