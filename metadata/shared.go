package metadata // import "bosun.org/metadata"

type HWDiskMeta struct {
	Name            string
	Media           string
	Capacity        string
	VendorId        string
	ProductId       string
	Serial          string
	Part            string
	NegotatiedSpeed string
	CapableSpeed    string
	SectorSize      string
}

type HWControllerMeta struct {
	Name            string
	SlotId          string
	State           string
	FirmwareVersion string
	DriverVersion   string
}

type HWPowerSupplyMeta struct {
	RatedInputWattage  string
	RatedOutputWattage string
}
