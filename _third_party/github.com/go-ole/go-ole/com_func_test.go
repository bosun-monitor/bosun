// +build !windows

package ole

import "testing"

func TestComSetupAndShutDown(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := coInitialize()
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestComPublicSetupAndShutDown(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := CoInitialize(0)
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestComPublicSetupAndShutDown_WithValue(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := CoInitialize(5)
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestComExSetupAndShutDown(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := coInitializeEx(COINIT_MULTITHREADED)
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestComPublicExSetupAndShutDown(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := CoInitializeEx(0, COINIT_MULTITHREADED)
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestComPublicExSetupAndShutDown_WithValue(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	err := CoInitializeEx(5, COINIT_MULTITHREADED)
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	CoUninitialize()
}

func TestClsidFromProgID_WindowsMediaNSSManager(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	coInitialize()
	defer CoUninitialize()
	_, err := CLSIDFromProgID("WMPNSSCI.NSSManager")
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}
}

func TestClsidFromString_WindowsMediaNSSManager(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	coInitialize()
	defer CoUninitialize()
	_, err := CLSIDFromString("{92498132-4D1A-4297-9B78-9E2E4BA99C07}")

	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}
}

func TestCreateInstance_WindowsMediaNSSManager(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	coInitialize()
	defer CoUninitialize()
	_, err := CLSIDFromProgID("WMPNSSCI.NSSManager")

	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}
}

func TestError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Fail()
		}
	}()

	coInitialize()
	defer CoUninitialize()
	_, err := CLSIDFromProgID("INTERFACE-NOT-FOUND")
	if err == nil {
		t.Error("should be error, because only Windows is supported.")
		t.FailNow()
	}

	switch vt := err.(type) {
	case *OleError:
	default:
		t.Fatalf("should be *ole.OleError %t", vt)
	}
}
