// Package statusio provides a client for the public v2 api of pages powered by statusio
package statusio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bosun.org/slog"
)

type Client struct {
	baseAddr string
	client   *http.Client
	scheme   string
}

// extractScheme detects if the url is http instead of https
func extractScheme(url string) (scheme, baseAddr string) {
	baseAddr = strings.TrimPrefix(url, "http://")
	if len(baseAddr) != len(url) {
		return "http", baseAddr
	}
	return "https", strings.TrimPrefix(url, "https://")
}

// NewClient creates a new statusio client for the *public api*.
// baseAddr is i.e. status.example.copm/api/v2/
func NewClient(baseAddr string) *Client {
	scheme, baseAddr := extractScheme(baseAddr)
	return &Client{
		baseAddr: baseAddr,
		client:   &http.Client{},
		scheme:   scheme,
	}
}

type SummaryResponse struct {
	Components            []Component            `json:"components"`
	Incidents             []Incident             `json:"incidents"`
	Page                  Page                   `json:"page"`
	ScheduledMaintenances []ScheduledMaintenance `json:"scheduled_maintenances"`
	Status                struct {
		Description     string `json:"description"`
		StatusIndicator string `json:"indicator"`
	} `json:"status"`
}

// GetSummary returns a summary of the status page, including a status indicator, component statuses, unresolved incidents, and any upcoming or in-progress scheduled maintenances.
func (c *Client) GetSummary(ctx context.Context) (SummaryResponse, error) {
	s := SummaryResponse{}
	err := c.request(ctx, "api/v2/summary.json", &s)
	return s, err
}

type StatusResponse struct {
	Page   Page `json:"page"`
	Status struct {
		Description string          `json:"description"`
		Indicator   StatusIndicator `json:"indicator"`
	} `json:"status"`
}

// GetStatus returns rollup for the whole page. This endpoint includes an indicator - one of none, minor, major, or critical, as well as a human description of the blended component status.
// Examples of the blended status include "All Systems Operational", "Partial System Outage", and "Major Service Outage".
func (c *Client) GetStatus(ctx context.Context) (StatusResponse, error) {
	s := StatusResponse{}
	err := c.request(ctx, "api/v2/status.json", &s)
	return s, err
}

type ComponentsResponse struct {
	Components []Component `json:"components"`
	Page       Page        `json:"page"`
}

// GetComponents gets the components for the page. Each component is listed along with its status - one of operational, degraded_performance, partial_outage, or major_outage.
func (c *Client) GetComponents(ctx context.Context) (ComponentsResponse, error) {
	comp := ComponentsResponse{}
	err := c.request(ctx, "api/v2/components.json", &comp)
	return comp, err
}

type IncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
	Page      Page       `json:"page"`
}

// GetUnresolvedIncidents gets a list of any unresolved incidents.
// This endpoint will only return incidents in the Investigating, Identified, or Monitoring state.
func (c *Client) GetUnresolvedIncidents(ctx context.Context) (IncidentsResponse, error) {
	i := IncidentsResponse{}
	err := c.request(ctx, "api/v2/incidents/unresolved.json", &i)
	return i, err
}

// GetIncidents returns a list of the 50 most recent incidents.
// This includes all unresolved incidents returned in GetUnresolvedIncidents, as well as those in the Resolved and Postmortem state.
func (c *Client) GetIncidents(ctx context.Context) (IncidentsResponse, error) {
	i := IncidentsResponse{}
	err := c.request(ctx, "api/v2/incidents.json", &i)
	return i, err
}

type ScheduledMaintenancesResponse struct {
	Page                  Page                   `json:"page"`
	ScheduledMaintenances []ScheduledMaintenance `json:"scheduled_maintenances"`
}

// GetUpcomingScheduledMaintenances gets a list of any upcoming maintenances.
// This endpoint will only return scheduled maintenances still in the Scheduled state.
func (c *Client) GetUpcomingScheduledMaintenances(ctx context.Context) (ScheduledMaintenancesResponse, error) {
	s := ScheduledMaintenancesResponse{}
	err := c.request(ctx, "api/v2/scheduled-maintenances/upcoming.json", &s)
	return s, err
}

// GetActiveScheduledMaintenances gets a list of any upcoming maintenances. // This endpoint will only return scheduled maintenances in the In Progress or Verifying state.
func (c *Client) GetActiveScheduledMaintenances(ctx context.Context) (ScheduledMaintenancesResponse, error) {
	s := ScheduledMaintenancesResponse{}
	err := c.request(ctx, "api/v2/scheduled-maintenances/active.json", &s)
	return s, err
}

// GetAllScheduledMaintenances gets a list of the 50 most recent scheduled maintenances.
// This includes scheduled maintenances as described in the above two endpoints, as well as those in the Completed state.
func (c *Client) GetAllScheduledMaintenances(ctx context.Context) (ScheduledMaintenancesResponse, error) {
	s := ScheduledMaintenancesResponse{}
	err := c.request(ctx, "api/v2/scheduled-maintenances.json", &s)
	return s, err
}

type NotImplemented bool

// GetPageSubscribers is not implemented
func (c *Client) GetPageSubscribers() NotImplemented {
	return true
}

// GetIncidentSubscribers is not implemented
func (c *Client) GetIncidentSubscribers() NotImplemented {
	return true
}

// RemoveSubscription is not implemented
func (c *Client) RemoveSubscription() NotImplemented {
	return true
}

func (c *Client) request(ctx context.Context, path string, s interface{}) error {
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.baseAddr,
		Path:   path,
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		slog.Error(err)
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%v: %v: %v", req.URL, resp.Status, string(b))
	}
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&s); err != nil {
		return err
	}
	return nil
}

// Page is part of all status.io public api responses.
type Page struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	UpdatedAt *time.Time `json:"updated_at"`
	URL       string     `json:"url"`
}

type Component struct {
	CreatedAt          *time.Time      `json:"created_at"`
	Description        interface{}     `json:"description"`
	Group              bool            `json:"group"`
	GroupID            interface{}     `json:"group_id"`
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	OnlyShowIfDegraded bool            `json:"only_show_if_degraded"`
	PageID             string          `json:"page_id"`
	Position           int             `json:"position"`
	Status             ComponentStatus `json:"status"`
	UpdatedAt          *time.Time      `json:"updated_at"`
}

type Incident struct {
	CreatedAt       *time.Time       `json:"created_at"`
	ID              string           `json:"id"`
	Impact          StatusIndicator  `json:"impact"`
	IncidentUpdates []IncidentUpdate `json:"incident_updates"`
	MonitoringAt    *time.Time       `json:"monitoring_at"`
	Name            string           `json:"name"`
	PageID          string           `json:"page_id"`
	ResolvedAt      *time.Time       `json:"resolved_at"`
	Shortlink       string           `json:"shortlink"`
	Status          IncidentStatus   `json:"status"`
	UpdatedAt       *time.Time       `json:"updated_at"`
}

type IncidentUpdate struct {
	Body       string     `json:"body"`
	CreatedAt  *time.Time `json:"created_at"`
	DisplayAt  *time.Time `json:"display_at"`
	ID         string     `json:"id"`
	IncidentID string     `json:"incident_id"`
	Status     string     `json:"status"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

type ScheduledMaintenance struct {
	CreatedAt       *time.Time        `json:"created_at"`
	ID              string            `json:"id"`
	Impact          StatusIndicator   `json:"impact"`
	IncidentUpdates []IncidentUpdate  `json:"incident_updates"`
	MonitoringAt    *time.Time        `json:"monitoring_at"`
	Name            string            `json:"name"`
	PageID          string            `json:"page_id"`
	ResolvedAt      *time.Time        `json:"resolved_at"`
	ScheduledFor    *time.Time        `json:"scheduled_for"`
	ScheduledUntil  *time.Time        `json:"scheduled_until"`
	Shortlink       string            `json:"shortlink"`
	Status          MaintenanceStatus `json:"status"`
	UpdatedAt       *time.Time        `json:"updated_at"`
}

type ComponentStatus int

func (c ComponentStatus) String() string {
	switch c {
	case 0:
		return "operational"
	case 1:
		return "degraded_performance"
	case 2:
		return "partial_outage"
	case 3:
		return "major_outage"
	}
	return "UnexpectedComponentStatus"
}

// Comments are the JSON representation
const (
	Operational         ComponentStatus = iota // operational
	DegradedPerformance                        // degraded_performance
	PartialOutage                              // partial_outage
	MajorOutage                                // major_outage
)

// ComponentStatusValues represents all the possible values of the ComponetStatus iota
var ComponentStatusValues = []ComponentStatus{Operational, DegradedPerformance, PartialOutage, MajorOutage}

// StatusIndicator is an enum used for general status and impact fields
type StatusIndicator int // ScheduledMaintenance Impact field seems to use the same enum

func (s StatusIndicator) String() string {
	switch s {
	case 0:
		return "none"
	case 1:
		return "minor"
	case 2:
		return "major"
	case 3:
		return "critical"
	}
	return "UnexpectedStatusIndicator"
}

// Comments are the JSON representation
const (
	None     StatusIndicator = iota // none
	Minor                           // minor
	Major                           // major
	Critical                        // critical
)

// StatusIndicatorValues represents all the possible values of the StatusIndicator iota
var StatusIndicatorValues = []StatusIndicator{None, Minor, Major, Critical}

// IncidentStatus represents the status of an incident
type IncidentStatus int

func (i IncidentStatus) String() string {
	switch i {
	case 0:
		return "investigating"
	case 1:
		return "identified"
	case 2:
		return "monitoring"
	case 3:
		return "resolved"
	case 4:
		return "post_mortem"
	}
	return "UnexpectedIncidentStatus"
}

// Comments are the JSON representation
const (
	Investigating IncidentStatus = iota // investigating
	Identified                          // identified
	Monitoring                          // monitoring
	Resolved                            // resolved
	PostMortem                          // post_mortem (?) Guessing, the documentation doesn't use the literals for this enum
)

// IncidentStatusValues represents all the possible values of an incident status
var IncidenStatusValues = []IncidentStatus{Investigating, Identified, Monitoring, Resolved, PostMortem}

// MaintenanceStatus represents the status of a maintenance operation
type MaintenanceStatus int

func (m MaintenanceStatus) String() string {
	switch m {
	case 0:
		return "scheduled"
	case 1:
		return "in_progress"
	case 2:
		return "verifying"
	case 3:
		return "completed"
	}
	return "UnexpectedMaintenanceStatus"
}

// Comments are the JSON representation
const (
	Scheduled  MaintenanceStatus = iota // scheduled
	InProgress                          // in_progress  (?) Guessing, the documentation doesn't use the literals for this enum
	Verifying                           // verifying
	Completed                           // completed
)

// MaitenanceStatusValues represents all the possible values of the MaintenanceStatus enum
var MaintenanceStatusValues = []MaintenanceStatus{Scheduled, InProgress, Verifying, Completed}
