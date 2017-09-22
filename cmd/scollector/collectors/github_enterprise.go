package collectors

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(startGithubEnterprise)
}

func startGithubEnterprise(c *conf.Conf) {
	for _, config := range c.GithubEnterprise {
		thisConfig := config
		if thisConfig.Instance != "" && thisConfig.SetupPassword != "" {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_ghe(&gheConf{
						Scollector: &thisConfig,
					})
				},
				Interval: 5 * time.Minute, //The vast majority of these values never change, but you might want to alert on unexpected maintenance mode quickly
				name:     "ghe",
			})
		}
	}
}

func c_ghe(gheconf *gheConf) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	if err := gheBaseSetting(gheconf, &md); err != nil {
		return nil, err
	}

	if err := gheMaintenanceSetting(gheconf, &md); err != nil {
		return nil, err
	}

	return md, nil
}

// gheBaseSetting is the base settings of the GitHub appliance. There's a lot of instance-specific configuration in here.
// Most of the configuration is not interesting from a monitoring standpoint, but there are certain things you may
// want to alert on if they change (e.g. if someone switches the instance to public, or the license count changes)
func gheBaseSetting(conf *gheConf, md *opentsdb.MultiDataPoint) error {
	var gheConfig gheBaseSettings
	resp, err := gheResponse(conf, "settings")
	if err != nil {
		//GHE returns a status 400 in the event that we querying an instance that is in replica mode. This replica may become active in the future,
		//so if we see a 400 here, just fail silently. We can still pull other API data out - just not settings data.
		if resp != nil && resp.StatusCode == 400 {
			return nil
		}
		return err
	}
	err = json.NewDecoder(resp.Body).Decode(&gheConfig)
	if err != nil {
		return err
	}

	conf.InstanceHostname = gheConfig.Enterprise.GithubHostname

	baseTags := gheRespBaseTags(resp, conf)

	Add(md, "ghe.privatemode", gheConfig.Enterprise.PrivateMode, baseTags, metadata.Bool, metadata.Count, "Is the GitHub Enterprise instance in private mode?")
	Add(md, "ghe.publicpages", gheConfig.Enterprise.PublicPages, baseTags, metadata.Bool, metadata.Count, "Is the GitHub Enterprise Pages access in public mode?")
	Add(md, "ghe.subdomainisolation", gheConfig.Enterprise.SubdomainIsolation, baseTags, metadata.Bool, metadata.Count, "Is Subdomain Isolation enabled?")
	Add(md, "ghe.signupenabled", gheConfig.Enterprise.SignupEnabled, baseTags, metadata.Bool, metadata.Count, "Can users sign themselves up to the GitHub Enterprise instance?")

	Add(md, "ghe.license.clustersupport", gheConfig.Enterprise.License.ClusterSupport, baseTags, metadata.Gauge, metadata.Bool, "")
	Add(md, "ghe.license.evaluation", gheConfig.Enterprise.License.Evaluation, baseTags, metadata.Gauge, metadata.Bool, "Is this an evaluation GitHub Enterprise license?")
	expiresAt, timeErr := time.Parse(time.RFC3339, gheConfig.Enterprise.License.ExpireAt)
	if timeErr == nil {
		expiresIn := time.Since(expiresAt).Seconds() * -1
		Add(md, "ghe.license.expireat", expiresIn, baseTags, metadata.Gauge, metadata.Second, "Number of seconds until the current GitHub Enterprise license expires")
	}
	Add(md, "ghe.license.perpetual", gheConfig.Enterprise.License.Perpetual, baseTags, metadata.Gauge, metadata.Bool, "Is this a perpetual GitHub Enterprise license?")
	Add(md, "ghe.license.seats", gheConfig.Enterprise.License.Seats, baseTags, metadata.Gauge, metadata.Count, "Number of licensed seats for GitHub Enterprise")
	Add(md, "ghe.license.sshallowed", gheConfig.Enterprise.License.SSHAllowed, baseTags, metadata.Gauge, metadata.Bool, "")
	Add(md, "ghe.license.unlimitedseating", gheConfig.Enterprise.License.UnlimitedSeating, baseTags, metadata.Gauge, metadata.Bool, "Is this an unlimited seats GitHub Enterprise license?")

	Add(md, "ghe.githubssl.enabled", gheConfig.Enterprise.GithubSsl.Enabled, baseTags, metadata.Gauge, metadata.Bool, "")

	// The following metrics are interesting, but not really useful. Commenting them out for now, but they're here if we want them in the future
	// Add(md, "ghe.abuseratelimiting.enabled", gheConfig.Enterprise.AbuseRateLimiting.Enabled, baseTags, metadata.Bool, metadata.Count, "")
	// Add(md, "ghe.abuseratelimiting.cpumillisperminute", gheConfig.Enterprise.AbuseRateLimiting.CPUMillisPerMinute, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.abuseratelimiting.requestsperminute", gheConfig.Enterprise.AbuseRateLimiting.RequestsPerMinute, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.abuseratelimiting.searchcpumillisperminute", gheConfig.Enterprise.AbuseRateLimiting.SearchCPUMillisPerMinute, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.enabled", gheConfig.Enterprise.APIRateLimiting.Enabled, baseTags, metadata.Gauge, metadata.Bool, "Is the API Rate Limit enforced?")
	// Add(md, "ghe.apiratelimit.defaultratelimit", gheConfig.Enterprise.APIRateLimiting.DefaultRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.graphqldefaultratelimit", gheConfig.Enterprise.APIRateLimiting.GraphqlDefaultRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.graphqlunauthenticatedratelimit", gheConfig.Enterprise.APIRateLimiting.GraphqlUnauthenticatedRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.lfsdefaultratelimit", gheConfig.Enterprise.APIRateLimiting.LfsDefaultRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.lfsunauthenticatedratelimit", gheConfig.Enterprise.APIRateLimiting.LfsUnauthenticatedRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.searchdefaultratelimit", gheConfig.Enterprise.APIRateLimiting.SearchDefaultRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.searchunauthenticatedratelimit", gheConfig.Enterprise.APIRateLimiting.SearchUnauthenticatedRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.apiratelimit.unauthenticatedratelimit", gheConfig.Enterprise.APIRateLimiting.UnauthenticatedRateLimit, baseTags, metadata.Gauge, metadata.Count, "")
	// Add(md, "ghe.govendor.quotasenabled", gheConfig.Enterprise.Governor.QuotasEnabled, baseTags, metadata.Gauge, metadata.Bool, "")

	return nil
}

// gheMaintenanceSetting gets the maintenance settings from the GitHub Enterprise API. Contains things like
// if maintenance mode is enabled, when it's scheduled for, etc.
func gheMaintenanceSetting(conf *gheConf, md *opentsdb.MultiDataPoint) error {
	var gheMaintSettings gheMaintenanceSettings
	resp, err := gheResponse(conf, "maintenance")
	if err != nil {
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(&gheMaintSettings)
	if err != nil {
		return err
	}

	baseTags := gheRespBaseTags(resp, conf)

	maintenanceMode := -1
	switch gheMaintSettings.Status {
	case "off":
		maintenanceMode = 0
	case "scheduled":
		maintenanceMode = 1
	case "on":
		maintenanceMode = 2
	}
	Add(md, "ghe.maintenance.status", maintenanceMode, baseTags, metadata.Gauge, metadata.Count, "Status of GitHub Enterprise maintenance. 0 = Off, 1 = Scheduled, 2 = On, -1 = Unknown")

	scheduledIn := float64(0)
	if gheMaintSettings.ScheduledTime != "" {
		scheduledAt, timeErr := time.Parse("Monday, January _2 at 15:04 -0700", gheMaintSettings.ScheduledTime)
		//There is no year in the date format provided by the API, so assume current year. Yes,
		//this will have a bug if you set the maintenance window over midnight new years eve. But if you're doing
		//GitHub maintenance on NYE, then you're probably in a Catherine Zeta-Jones movie featuring Sean Connory.
		scheduledAt = scheduledAt.AddDate(time.Now().Year(), 0, 0)
		if timeErr == nil {
			scheduledIn = time.Since(scheduledAt).Seconds() * -1
		}
	}

	Add(md, "ghe.maintenance.scheduled", scheduledIn, baseTags, metadata.Gauge, metadata.Second, "The number of seconds until GitHub Enterprise Maintenance mode is enabled")

	for _, cs := range gheMaintSettings.ConnectionServices {
		theseTags := baseTags
		theseTags["servicename"] = cs.Name
		Add(md, "ghe.maintenance.connectionservices", cs.Number, theseTags, metadata.Gauge, metadata.Count, "")
	}

	return nil
}

// gheRespBaseTags return the set of tags that every metric should have. By the time this is called for
// the first time, it's assumed that the base settings have been retrieved (this is where the
// Instance Hostname comes from)
func gheRespBaseTags(resp *http.Response, conf *gheConf) opentsdb.TagSet {
	hostname := strings.Split(resp.Request.Host, ":")[0]
	baseTags := opentsdb.TagSet{
		"host": hostname,
	}
	if conf.InstanceHostname != "" {
		baseTags["hostname"] = conf.InstanceHostname
	}
	return baseTags
}

// gheResponse returns the response of an API query to the setup API. Basically it just does a normal
// HTTP response, but is sets the authorization header with the API key. The API key is actually the
// setup password, so treat with care.
// NOTE: SSL validation is disabled here, because if the certificate that GitHub Enterprise is
// configured with is not the same as the hostname you are accessing (e.g. if you have more than one
// github installation serving the same clustered instance) the HTTP request will otherwise fail.
func gheResponse(conf *gheConf, endpoint string) (*http.Response, error) {
	baseURI, err := url.Parse(conf.Scollector.Instance)
	if err != nil {
		return nil, err
	}
	baseURI.Path = path.Join("setup/api/", endpoint)
	req, err := http.NewRequest("GET", baseURI.String(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("api_key", conf.Scollector.SetupPassword)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return resp, fmt.Errorf("Status code %v returned - expected 200", resp.StatusCode)
	}

	return resp, nil
}

type gheConf struct {
	Scollector       *conf.GithubEnterprise
	InstanceHostname string
}

type gheBaseSettings struct {
	Enterprise struct {
		PrivateMode           bool        `json:"private_mode"`
		PublicPages           bool        `json:"public_pages"`
		SubdomainIsolation    bool        `json:"subdomain_isolation"`
		SignupEnabled         bool        `json:"signup_enabled"`
		GithubHostname        string      `json:"github_hostname"`
		IdenticonsHost        string      `json:"identicons_host"`
		HTTPProxy             interface{} `json:"http_proxy"`
		HTTPNoproxy           interface{} `json:"http_noproxy"`
		AuthMode              string      `json:"auth_mode"`
		ExpireSessions        bool        `json:"expire_sessions"`
		AdminPassword         interface{} `json:"admin_password"`
		ConfigurationID       int         `json:"configuration_id"`
		ConfigurationRunCount int         `json:"configuration_run_count"`
		Avatar                interface{} `json:"avatar"`
		Customer              struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			UUID  string `json:"uuid"`
		} `json:"customer"`
		License struct {
			Seats            int         `json:"seats"`
			Evaluation       bool        `json:"evaluation"`
			Perpetual        bool        `json:"perpetual"`
			UnlimitedSeating bool        `json:"unlimited_seating"`
			SupportKey       interface{} `json:"support_key"`
			SSHAllowed       bool        `json:"ssh_allowed"`
			ClusterSupport   bool        `json:"cluster_support"`
			ExpireAt         string      `json:"expire_at"`
		} `json:"license"`
		GithubSsl struct {
			Enabled bool     `json:"enabled"`
			TLSMode []string `json:"tls_mode"`
		} `json:"github_ssl"`
		Ldap struct {
			Host                      string      `json:"host"`
			Port                      int         `json:"port"`
			Base                      []string    `json:"base"`
			UID                       interface{} `json:"uid"`
			BindDn                    string      `json:"bind_dn"`
			Method                    string      `json:"method"`
			SearchStrategy            string      `json:"search_strategy"`
			UserGroups                []string    `json:"user_groups"`
			AdminGroup                string      `json:"admin_group"`
			VirtualAttributeEnabled   bool        `json:"virtual_attribute_enabled"`
			RecursiveGroupSearch      bool        `json:"recursive_group_search"`
			PosixSupport              bool        `json:"posix_support"`
			UserSyncEmails            bool        `json:"user_sync_emails"`
			UserSyncKeys              bool        `json:"user_sync_keys"`
			UserSyncGpgKeys           bool        `json:"user_sync_gpg_keys"`
			UserSyncInterval          int         `json:"user_sync_interval"`
			TeamSyncInterval          int         `json:"team_sync_interval"`
			SyncEnabled               bool        `json:"sync_enabled"`
			ExternalAuthTokenRequired bool        `json:"external_auth_token_required"`
			VerifyCertificate         bool        `json:"verify_certificate"`
			Reconciliation            struct {
				User interface{} `json:"user"`
				Org  interface{} `json:"org"`
			} `json:"reconciliation"`
			Profile struct {
				UID    string `json:"uid"`
				Name   string `json:"name"`
				Mail   string `json:"mail"`
				Key    string `json:"key"`
				GpgKey string `json:"gpg_key"`
			} `json:"profile"`
		} `json:"ldap"`
		Cas struct {
			URL interface{} `json:"url"`
		} `json:"cas"`
		Saml struct {
			SsoURL             interface{} `json:"sso_url"`
			Certificate        interface{} `json:"certificate"`
			CertificatePath    string      `json:"certificate_path"`
			Issuer             interface{} `json:"issuer"`
			NameIDFormat       string      `json:"name_id_format"`
			IdpInitiatedSso    bool        `json:"idp_initiated_sso"`
			DisableAdminDemote bool        `json:"disable_admin_demote"`
			SignatureMethod    string      `json:"signature_method"`
			DigestMethod       string      `json:"digest_method"`
			UsernameAttribute  interface{} `json:"username_attribute"`
			FullNameAttribute  string      `json:"full_name_attribute"`
			EmailsAttribute    string      `json:"emails_attribute"`
			SSHKeysAttribute   string      `json:"ssh_keys_attribute"`
			GpgKeysAttribute   string      `json:"gpg_keys_attribute"`
		} `json:"saml"`
		GithubOauth interface{} `json:"github_oauth"`
		SMTP        struct {
			Enabled            bool        `json:"enabled"`
			Address            string      `json:"address"`
			Authentication     interface{} `json:"authentication"`
			Port               int         `json:"port"`
			Domain             string      `json:"domain"`
			Username           interface{} `json:"username"`
			UserName           interface{} `json:"user_name"`
			SupportAddress     string      `json:"support_address"`
			SupportAddressType string      `json:"support_address_type"`
			NoreplyAddress     string      `json:"noreply_address"`
		} `json:"smtp"`
		Ntp struct {
			PrimaryServer   string `json:"primary_server"`
			SecondaryServer string `json:"secondary_server"`
		} `json:"ntp"`
		Timezone interface{} `json:"timezone"`
		Snmp     struct {
			Enabled   bool          `json:"enabled"`
			Version   int           `json:"version"`
			Community string        `json:"community"`
			Users     []interface{} `json:"users"`
		} `json:"snmp"`
		Syslog struct {
			Enabled      bool        `json:"enabled"`
			Server       interface{} `json:"server"`
			ProtocolName string      `json:"protocol_name"`
			TLSEnabled   bool        `json:"tls_enabled"`
			Cert         interface{} `json:"cert"`
		} `json:"syslog"`
		Assets interface{} `json:"assets"`
		Pages  struct {
			Enabled bool `json:"enabled"`
		} `json:"pages"`
		Collectd struct {
			Enabled    bool        `json:"enabled"`
			Server     interface{} `json:"server"`
			Port       int         `json:"port"`
			Encryption interface{} `json:"encryption"`
			Username   interface{} `json:"username"`
		} `json:"collectd"`
		Mapping struct {
			Enabled    bool        `json:"enabled"`
			Tileserver interface{} `json:"tileserver"`
			Basemap    string      `json:"basemap"`
			Token      interface{} `json:"token"`
		} `json:"mapping"`
		LoadBalancer struct {
			HTTPForward   bool `json:"http_forward"`
			ProxyProtocol bool `json:"proxy_protocol"`
		} `json:"load_balancer"`
		AbuseRateLimiting struct {
			Enabled                  bool `json:"enabled"`
			RequestsPerMinute        int  `json:"requests_per_minute"`
			CPUMillisPerMinute       int  `json:"cpu_millis_per_minute"`
			SearchCPUMillisPerMinute int  `json:"search_cpu_millis_per_minute"`
		} `json:"abuse_rate_limiting"`
		APIRateLimiting struct {
			Enabled                         bool `json:"enabled"`
			UnauthenticatedRateLimit        int  `json:"unauthenticated_rate_limit"`
			DefaultRateLimit                int  `json:"default_rate_limit"`
			SearchUnauthenticatedRateLimit  int  `json:"search_unauthenticated_rate_limit"`
			SearchDefaultRateLimit          int  `json:"search_default_rate_limit"`
			LfsUnauthenticatedRateLimit     int  `json:"lfs_unauthenticated_rate_limit"`
			LfsDefaultRateLimit             int  `json:"lfs_default_rate_limit"`
			GraphqlUnauthenticatedRateLimit int  `json:"graphql_unauthenticated_rate_limit"`
			GraphqlDefaultRateLimit         int  `json:"graphql_default_rate_limit"`
		} `json:"api_rate_limiting"`
		Governor struct {
			QuotasEnabled bool        `json:"quotas_enabled"`
			LimitUser     interface{} `json:"limit_user"`
			LimitNetwork  interface{} `json:"limit_network"`
		} `json:"governor"`
	} `json:"enterprise"`
	RunList []string `json:"run_list"`
}

type gheMaintenanceSettings struct {
	Status             string `json:"status"`
	ScheduledTime      string `json:"scheduled_time"`
	ConnectionServices []struct {
		Name   string `json:"name"`
		Number string `json:"number"`
	} `json:"connection_services"`
}
