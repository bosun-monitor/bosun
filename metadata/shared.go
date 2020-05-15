package metadata // import "bosun.org/metadata"

// HWDiskMeta is a struct representing disk metadata
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

// HWControllerMeta is a struct representing storage controller metadata
type HWControllerMeta struct {
	Name            string
	SlotId          string
	State           string
	FirmwareVersion string
	DriverVersion   string
}

// HWPowerSupplyMeta is a struct representing power supply metadata
type HWPowerSupplyMeta struct {
	RatedInputWattage  string
	RatedOutputWattage string
}
