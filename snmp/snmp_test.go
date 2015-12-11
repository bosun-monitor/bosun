// +build ignore

package snmp

const (
	stringType    = "SNMPv2-MIB::sysDescr.0"
	oidType       = "SNMPv2-MIB::sysObjectID.0"
	timeticksType = "HOST-RESOURCES-MIB::hrSystemUptime.0"
	counter32Type = "IF-MIB::ifOutOctets.1"
	counter64Type = "IF-MIB::ifHCOutOctets.1"
	gauge32Type   = "IF-MIB::ifSpeed.1"
)
