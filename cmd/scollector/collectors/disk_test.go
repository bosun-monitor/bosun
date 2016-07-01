package collectors

import (
	"strings"
	"testing"
)

func TestGetResync(t *testing.T) {
	progress, ok := getResync("Rebuild Status : 0%")
	if progress >= 0.0001 || !ok {
		t.Fatalf("Rebuild Status should be 0 percent, got %f", progress)
	}
	progress, ok = getResync("Rebuild Status : 20% complete")
	if progress != 20.0 || !ok {
		t.Fatalf("progress should be at 20 percent got: %f", progress)
	}
	if _, ok = getResync("blam 20% dd"); ok {
		t.Fatalf("we should not have detected a resync")
	}
}

type spindle struct {
	dev string
	ok  bool
}

func TestGetSpindle(t *testing.T) {
	tests := map[string]spindle{
		"2       8       19        0      active sync   /dev/sdb3": {"/dev/sdb3", true},
		"2       0        0        2      removed":                 {"", false},
		"3       8       17        -      faulty   /dev/sdb1":      {"/dev/sdb1", true},
		"fdaf /dev ": {"", false},
		"fdsa":       {"", false},
		"":           {"", false},
	}
	for s, expected := range tests {
		if dev, ok := getSpindle(s); ok != expected.ok || dev != expected.dev {
			t.Fatalf("Wrong spindle result: got: %v %v, expected: %v %v", dev, ok, expected.dev, expected.ok)
		}
	}
}

type state struct {
	s  int
	ok bool
}

func TestGetState(t *testing.T) {
	tests := map[string]state{
		"State : clean, degraded": {mdadmDegraded, true},
		"State : active, FAILED":  {mdadmFailed, true},
		"State : clean":           {mdadmNormal, true},
		" fdaf /dev ":             {mdadmUnknown, false},
	}
	for s, expected := range tests {
		if state, ok := getState(s); ok != expected.ok || state != expected.s {
			t.Fatalf("Wrong state: got: %v %v, expected: %v %v", state, ok, expected.s, expected.ok)
		}
	}
}

func compTab(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func gotexp(got volumeDetail, exp volumeDetail, t *testing.T) {
	if got.syncProgress != exp.syncProgress {
		t.Fatalf("parseExamineMdadm:syncProgress got: %v exp: %v", got.syncProgress, exp.syncProgress)
	}
	if got.state != exp.state {
		t.Fatalf("parseExamineMdadm:syncProgress got: %v exp: %v", got.state, exp.state)
	}
	if !compTab(got.failedSpindle, exp.failedSpindle) {
		t.Fatalf("parseExamineMdadm:syncProgress got: %v exp: %v", got.failedSpindle, exp.failedSpindle)
	}
	if !compTab(got.activeSpindle, exp.activeSpindle) {
		t.Fatalf("parseExamineMdadm:syncProgress got: %v exp: %v", got.activeSpindle, exp.activeSpindle)
	}
	if !compTab(got.spareSpindle, exp.spareSpindle) {
		t.Fatalf("parseExamineMdadm:syncProgress got: %v exp: %v", got.spareSpindle, exp.spareSpindle)
	}
}

func TestParseExamineMdadm(t *testing.T) {
	test := ` /dev/md0:
        Version : 1.2
  Creation Time : Tue Jun 28 20:11:03 2016
     Raid Level : raid1
     Array Size : 204608 (199.85 MiB 209.52 MB)
  Used Dev Size : 204608 (199.85 MiB 209.52 MB)
   Raid Devices : 2
  Total Devices : 2
    Persistence : Superblock is persistent

    Update Time : Wed Jun 29 20:40:09 2016
          State : clean, degraded
 Active Devices : 1
Working Devices : 1
 Failed Devices : 1
  Spare Devices : 0

           Name : ny-xav01.ds.stackexchange.com:0  (local to host ny-xav01.ds.stackexchange.com)
           UUID : 92a9102d:e9e62da2:ed2ed365:07251d1b
         Events : 63

    Number   Major   Minor   RaidDevice State
       2       8       19        0      active sync   /dev/sdb3
       2       0        0        2      removed

       3       8       17        -      faulty   /dev/sdb1
`
	exp := volumeDetail{
		100.0,
		mdadmDegraded,
		[]string{"/dev/sdb1"},
		[]string{"/dev/sdb3"},
		[]string{},
	}

	got := parseExamineMdadm(strings.NewReader(test))
	gotexp(got, exp, t)

	t2 := `
/dev/md0:
        Version : 1.2
  Creation Time : Tue Jun 28 20:11:03 2016
     Raid Level : raid1
     Array Size : 204608 (199.85 MiB 209.52 MB)
  Used Dev Size : 204608 (199.85 MiB 209.52 MB)
   Raid Devices : 2
  Total Devices : 2
    Persistence : Superblock is persistent

    Update Time : Thu Jun 30 18:52:45 2016
          State : clean, degraded, recovering
 Active Devices : 1
Working Devices : 2
 Failed Devices : 0
  Spare Devices : 1

 Rebuild Status : 29% complete

           Name : ny-xav01.ds.stackexchange.com:0  (local to host ny-xav01.ds.stackexchange.com)
           UUID : 92a9102d:e9e62da2:ed2ed365:07251d1b
         Events : 92

    Number   Major   Minor   RaidDevice State
       2       8       19        0      active sync   /dev/sdb3
       3       8       17        1      spare rebuilding   /dev/sdb1
`
	exp = volumeDetail{
		29.0,
		mdadmDegraded,
		[]string{},
		[]string{"/dev/sdb3"},
		[]string{"/dev/sdb1"},
	}

	got = parseExamineMdadm(strings.NewReader(t2))
	gotexp(got, exp, t)

	t3 := `/dev/md0:
        Version : 1.2
  Creation Time : Tue Jun 28 20:11:03 2016
     Raid Level : raid1
     Array Size : 204608 (199.85 MiB 209.52 MB)
  Used Dev Size : 204608 (199.85 MiB 209.52 MB)
   Raid Devices : 2
  Total Devices : 2
    Persistence : Superblock is persistent

    Update Time : Thu Jun 30 18:55:13 2016
          State : clean
 Active Devices : 2
Working Devices : 2
 Failed Devices : 0
  Spare Devices : 0

           Name : ny-xav01.ds.stackexchange.com:0  (local to host ny-xav01.ds.stackexchange.com)
           UUID : 92a9102d:e9e62da2:ed2ed365:07251d1b
         Events : 105

    Number   Major   Minor   RaidDevice State
       2       8       19        0      active sync   /dev/sdb3
       3       8       17        1      active sync   /dev/sdb1
`

	exp = volumeDetail{
		100.0,
		mdadmNormal,
		[]string{},
		[]string{"/dev/sdb3", "/dev/sdb1"},
		[]string{},
	}
	got = parseExamineMdadm(strings.NewReader(t3))
	gotexp(got, exp, t)
}
