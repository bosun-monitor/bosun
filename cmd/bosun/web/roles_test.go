package web

import (
	"testing"

	"github.com/captncraig/easyauth"
)

func TestRoleParse(t *testing.T) {
	tests := []struct {
		s      string
		expect easyauth.Role
		errors bool
	}{
		{"ViewDashboard", canViewDash, false},
		{"ViewDasHBoard", canViewDash, false},
		{"ViewDashboardzzzz", 0, true},
		{"Admin", roleAdmin, false},
		{"ViewDashboard,PutData", canViewDash | canPutData, false},
		{"ViewDashboard,Thisdoesnotexist", 0, true},
	}
	for i, test := range tests {
		found, err := parseRole(test.s)
		if err != nil && !test.errors {
			t.Errorf("%d: Unexpected error for %s", i, test.s)
			continue
		}
		if err == nil && test.errors {
			t.Errorf("%d: Expected error not found for %s", i, test.s)
			continue
		}
		if found != test.expect {
			t.Errorf("%d: Expected %d but got %d", i, test.expect, found)
		}
	}
}
