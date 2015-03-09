// +build windows

package ole

import (
	_ "fmt"
	"testing"
	_ "unsafe"
)

// This tests more than one function. It tests all of the functions needed in order to retrieve an
// SafeArray populated with Strings.
func TestGetSafeArrayString(t *testing.T) {
	CoInitialize(0)
	defer CoUninitialize()

	clsid, err := CLSIDFromProgID("QBXMLRP2.RequestProcessor.1")
	if err != nil {
		if err.(*OleError).Code() == CO_E_CLASSSTRING {
			return
		}
		t.Log(err)
		t.FailNow()
	}

	unknown, err := CreateInstance(clsid, IID_IUnknown)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer unknown.Release()

	dispatch, err := unknown.QueryInterface(IID_IDispatch)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var dispid []int32
	dispid, err = dispatch.GetIDsOfName([]string{"OpenConnection2"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var result *VARIANT
	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, "", "Test Application 1", 1)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	dispid, err = dispatch.GetIDsOfName([]string{"BeginSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	result, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, "", 2)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	ticket := result.ToString()

	dispid, err = dispatch.GetIDsOfName([]string{"QBXMLVersionsForSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	result, err = dispatch.Invoke(dispid[0], DISPATCH_PROPERTYGET, ticket)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// Where the real tests begin.
	var qbXMLVersions *SafeArray
	var qbXmlVersionStrings []string
	qbXMLVersions = result.ToArray().Array

	// Get array bounds
	var LowerBounds int64
	var UpperBounds int64
	LowerBounds, err = safeArrayGetLBound(qbXMLVersions, 1)
	if err != nil {
		t.Log("Safe Array Get Lower Bound")
		t.Log(err)
		t.FailNow()
	}
	t.Log("Lower Bounds:")
	t.Log(LowerBounds)

	UpperBounds, err = safeArrayGetUBound(qbXMLVersions, 1)
	if err != nil {
		t.Log("Safe Array Get Lower Bound")
		t.Log(err)
		t.FailNow()
	}
	t.Log("Upper Bounds:")
	t.Log(UpperBounds)

	totalElements := UpperBounds - LowerBounds + 1
	qbXmlVersionStrings = make([]string, totalElements)

	for i := int64(0); i < totalElements; i++ {
		qbXmlVersionStrings[int32(i)], _ = safeArrayGetElementString(qbXMLVersions, i)
	}

	// Release Safe Array memory
	safeArrayDestroy(qbXMLVersions)

	dispid, err = dispatch.GetIDsOfName([]string{"EndSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, ticket)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	dispid, err = dispatch.GetIDsOfName([]string{"CloseConnection"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}
