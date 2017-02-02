package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/captncraig/easyauth"
)

const (
	canViewDash easyauth.Role = 1 << iota
	canViewConfig
	canPutData
	canPerformActions
	canRunTests
	canSaveConfig
	canViewAnnotations
	canCreateAnnotations
	canSilence
	canManageTokens
	canOverwriteUsername
)

const (
	fullyOpen  easyauth.Role = 0
	roleReader               = canViewDash | canViewConfig | canViewAnnotations
	roleAdmin                = 0xFFFFFFFF
	roleWriter               = roleAdmin ^ canManageTokens ^ canOverwriteUsername
)

var roleDefs = &roleMetadata{
	Permissions: []bitDesc{
		{canViewDash, "View Dashboard", "Can view dashboard and alert state data, metrics, and graphs"},
		{canViewConfig, "View Config", "Can view bosun configuration page"},
		{canPutData, "Put Data", "Can put and index opentsdb data and metadata"},
		{canPerformActions, "Actions", "Can acknowlege and close alerts"},
		{canRunTests, "Run Tests", "Can execute expressions, graphs, and rule tests"},
		{canSaveConfig, "Save Config", "Can alter and save bosun rule config"},
		{canViewAnnotations, "View Annotations", "Can view annotations on graph page"},
		{canCreateAnnotations, "Create Annotations", "Can add and manage annotations via api"},
		{canSilence, "Silence", "Can add and manage silences"},
		{canManageTokens, "Manage Tokens", "Can manage authorization tokens"},
		{canOverwriteUsername, "Set Username", "Allows external services to set username in api requests"},
	},
	Roles: []bitDesc{
		{roleReader, "Reader", "Read access to dashboard and alert data"},
		{roleAdmin, "Admin", "Full access to all functionality"},
		{roleWriter, "Writer", "Write access to alert data and actions"},
	},
}

type bitDesc struct {
	Bits easyauth.Role
	Name string
	Desc string
}
type roleMetadata struct {
	Permissions []bitDesc
	Roles       []bitDesc
}

func parseRole(s string) (easyauth.Role, error) {
	parts := strings.Split(s, ",")
	perms := fullyOpen
	for _, part := range parts {
		this := fullyOpen
		for _, perm := range roleDefs.Permissions {
			pname := strings.Replace(strings.ToLower(perm.Name), " ", "", -1)
			if strings.ToLower(part) == pname {
				this = perm.Bits
				break
			}
		}
		for _, perm := range roleDefs.Roles {
			pname := strings.Replace(strings.ToLower(perm.Name), " ", "", -1)
			if strings.ToLower(part) == pname {
				this = perm.Bits
				break
			}
		}
		if this == fullyOpen {
			return this, fmt.Errorf("Unknown permission level: '%s'", part)
		}
		perms |= this
	}
	return perms, nil
}

func getRoleDefinitions(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return roleDefs, nil
}
