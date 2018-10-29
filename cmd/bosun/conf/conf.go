package conf // import "bosun.org/cmd/bosun/conf"

import (
	"bytes"
	"fmt"
	"hash/fnv"

	"net/mail"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/influxdata/influxdb/client/v2"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/graphite"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

// SystemConfProvider providers all the information about the system configuration.
// the interface exists to ensure that no changes are made to the system configuration
// outside of the package without a setter
type SystemConfProvider interface {
	GetHTTPListen() string
	GetHTTPSListen() string
	GetTLSCertFile() string
	GetTLSKeyFile() string

	GetRuleVars() map[string]string

	GetSMTPHost() string
	GetSMTPUsername() string // SMTP username
	GetSMTPPassword() string // SMTP password
	GetPing() bool
	GetPingDuration() time.Duration
	GetEmailFrom() string
	GetLedisDir() string
	GetLedisBindAddr() string
	GetRedisHost() string
	GetRedisDb() int
	GetRedisPassword() string
	GetTimeAndDate() []int
	GetSearchSince() time.Duration

	GetCheckFrequency() time.Duration
	GetDefaultRunEvery() int
	GetAlertCheckDistribution() string
	GetUnknownThreshold() int
	GetMinGroupSize() int

	GetShortURLKey() string
	GetInternetProxy() string

	GetRuleFilePath() string
	SaveEnabled() bool
	ReloadEnabled() bool
	GetCommandHookPath() string

	SetTSDBHost(tsdbHost string)
	GetTSDBHost() string

	GetAnnotateElasticHosts() expr.ElasticConfig
	GetAnnotateIndex() string

	GetAuthConf() *AuthConf

	GetMaxRenderedTemplateAge() int

	GetExampleExpression() string

	// Contexts
	GetTSDBContext() opentsdb.Context
	GetGraphiteContext() graphite.Context
	GetInfluxContext() client.HTTPConfig
	GetElasticContext() expr.ElasticHosts
	GetAzureMonitorContext() expr.AzureMonitorClients
	AnnotateEnabled() bool

	MakeLink(string, *url.Values) string
	EnabledBackends() EnabledBackends
}

// ValidateSystemConf runs sanity checks on the system configuration
func ValidateSystemConf(sc SystemConfProvider) error {
	hasSMTPHost := sc.GetSMTPHost() != ""
	hasEmailFrom := sc.GetEmailFrom() != ""
	if hasSMTPHost != hasEmailFrom {
		return fmt.Errorf("email notififications require that both SMTP Host and EmailFrom be set")
	}
	if sc.GetDefaultRunEvery() <= 0 {
		return fmt.Errorf("default run every must be greater than 0, is %v", sc.GetDefaultRunEvery())
	}
	if sc.GetHTTPSListen() != "" && (sc.GetTLSCertFile() == "" || sc.GetTLSKeyFile() == "") {
		return fmt.Errorf("must specify TLSCertFile and TLSKeyFile if HTTPSListen is specified")
	}
	return nil
}

// RuleConfProvider is an interface for accessing information that bosun needs to know about
// rule configuration. Rule configuration includes Macros, Alerts, Notifications, Lookup
// tables, squelching, and variable expansion. Currently there is only one implementation of
// this inside bosun in the rule package. The interface exists to ensure that the rest of
// Bosun does not manipulate the rule configuration in unexpected ways. Also so the possibility
// of an alternative store for rules can exist the future. However, when this is added it is expected
// that the interface will change significantly.
type RuleConfProvider interface {
	RuleConfWriter
	GetTemplate(string) *Template

	GetAlerts() map[string]*Alert
	GetAlert(string) *Alert

	GetNotifications() map[string]*Notification
	GetNotification(string) *Notification

	GetLookup(string) *Lookup

	AlertSquelched(*Alert) func(opentsdb.TagSet) bool
	Squelched(*Alert, opentsdb.TagSet) bool
	Expand(string, map[string]string, bool) string
	GetFuncs(EnabledBackends) map[string]parse.Func
}

// RuleConfWriter is a collection of the methods that are used to manipulate the configuration
// Save methods will trigger the reload that has been passed to the rule configuration
type RuleConfWriter interface {
	BulkEdit(BulkEditRequest) error
	GetRawText() string
	GetHash() string
	SaveRawText(rawConf, diff, user, message string, args ...string) error
	RawDiff(rawConf string) (string, error)
	SetReload(reload func() error)
	SetSaveHook(SaveHook)
}

// Squelch is a map of tag keys to regexes that are applied to tag values. Squelches
// are used to filter results from query responses
type Squelch map[string]*regexp.Regexp

// Squelches is a collection of Squelch
type Squelches []Squelch

// Add adds a sqluech baed on the tags in the first argument. The value of the tag
// is a regular expression. Tags are passed as a string in the format of
func (s *Squelches) Add(v string) error {
	tags, err := opentsdb.ParseTags(v)
	if tags == nil && err != nil {
		return err
	}
	sq := make(Squelch)
	for k, v := range tags {
		re, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		sq[k] = re
	}
	*s = append(*s, sq)
	return nil
}

// Squelched takes a tag set and returns true if the given
// tagset should be squelched based on the Squelches
func (s *Squelches) Squelched(tags opentsdb.TagSet) bool {
	for _, squelch := range *s {
		if squelch.Squelched(tags) {
			return true
		}
	}
	return false
}

// Squelched takes a tag set and returns true if the given
// tagset should be squelched based on the Squelche
func (s Squelch) Squelched(tags opentsdb.TagSet) bool {
	if len(s) == 0 {
		return false
	}
	for k, v := range s {
		tagv, ok := tags[k]
		if !ok || !v.MatchString(tagv) {
			return false
		}
	}
	return true
}

// Template stores information about a notification template. Templates
// are based on Go's text and html/template.
type Template struct {
	Text string
	Vars
	Name            string
	Body            *template.Template            `json:"-"`
	Subject         *template.Template            `json:"-"`
	CustomTemplates map[string]*template.Template `json:"-"`

	RawBody, RawSubject string
	RawCustoms          map[string]string

	Locator `json:"-"`
}

func (t *Template) Get(name string) *template.Template {
	if name == "body" {
		return t.Body
	}
	if name == "subject" {
		return t.Subject
	}
	return t.CustomTemplates[name]
}

// NotificationTemplateKeys is the set of fields that may be templated out per notification. Each field points to the name of a field on a template object.
type NotificationTemplateKeys struct {
	PostTemplate, GetTemplate string // templates to use for post/get urls
	BodyTemplate              string // template to use for post body or email body. defaults to "body" for post and "emailBody" (if it exists) for email
	EmailSubjectTemplate      string // template to use for email subject. Default to "subject"
}

// Combine merges keys from another set, copying only those values that do not exist on the first set of template keys.
// It returns a new object every time, and accepts nils on either side.
func (n *NotificationTemplateKeys) Combine(defaults *NotificationTemplateKeys) *NotificationTemplateKeys {
	n2 := &NotificationTemplateKeys{}
	if n != nil {
		n2.PostTemplate = n.PostTemplate
		n2.GetTemplate = n.GetTemplate
		n2.BodyTemplate = n.BodyTemplate
		n2.EmailSubjectTemplate = n.EmailSubjectTemplate
	}
	if defaults == nil {
		return n2
	}
	if n2.PostTemplate == "" {
		n2.PostTemplate = defaults.PostTemplate
	}
	if n2.GetTemplate == "" {
		n2.GetTemplate = defaults.GetTemplate
	}
	if n2.BodyTemplate == "" {
		n2.BodyTemplate = defaults.BodyTemplate
	}
	if n2.EmailSubjectTemplate == "" {
		n2.EmailSubjectTemplate = defaults.EmailSubjectTemplate
	}
	return n2
}

// Notification stores information about a notification. A notification
// is the definition of an action that should be performed when an
// alert is triggered
type Notification struct {
	Text string
	Vars
	Name  string
	Email []*mail.Address

	Post, Get *url.URL

	// template keys to use for plain notifications
	NotificationTemplateKeys

	// template keys to use for action notifications. ActionNone contains catch-all fields if present. More specific will override.
	ActionTemplateKeys map[models.ActionType]*NotificationTemplateKeys

	UnknownTemplateKeys      NotificationTemplateKeys
	UnknownMultiTemplateKeys NotificationTemplateKeys

	Print        bool
	Next         *Notification
	Timeout      time.Duration
	ContentType  string
	RunOnActions string
	GroupActions bool

	UnknownMinGroupSize *int // nil means use global defaults. 0 means no-grouping at all.
	UnknownThreshold    *int // nil means use global defaults. 0 means no limit

	NextName        string `json:"-"`
	RawEmail        string `json:"-"`
	RawPost, RawGet string `json:"-"`

	Locator `json:"-"`
}

// Vars holds a map of variable names to the variable's value
type Vars map[string]string

// Notifications contains a mapping of notification names to
// all notifications in the configuration. The Lookups Property
// enables notification lookups - the ability to trigger different
// notifications based an alerts resulting tags
type Notifications struct {
	Notifications map[string]*Notification `json:"-"`
	// Table key -> table
	Lookups map[string]*Lookup
}

// Get returns the set of notifications based on given tags and applys any notification
// lookup tables
func (ns *Notifications) Get(c RuleConfProvider, tags opentsdb.TagSet) map[string]*Notification {
	nots := make(map[string]*Notification)
	for name, n := range ns.Notifications {
		nots[name] = n
	}
	for key, lookup := range ns.Lookups {
		l := lookup.ToExpr()
		val, ok := l.Get(key, tags)
		if !ok {
			continue
		}
		ns := make(map[string]*Notification)
		for _, s := range strings.Split(val, ",") {
			s = strings.TrimSpace(s)
			n := c.GetNotification(s)
			if n == nil {
				continue // TODO error here?
			}
			ns[s] = n
		}
		for name, n := range ns {
			nots[name] = n
		}
	}
	return nots
}

// GetAllChained returns all unique notifications, including chains
func (ns *Notifications) GetAllChained() map[string]*Notification {
	m := map[string]*Notification{}
	var walk func(not *Notification)
	walk = func(not *Notification) {
		if m[not.Name] != nil {
			return
		}
		m[not.Name] = not
		if not.Next != nil {
			walk(not.Next)
		}
	}
	for _, not := range ns.Notifications {
		walk(not)
	}
	return m
}

// GetNotificationChains returns the warn or crit notification chains for a configured
// alert. Each chain is a list of notification names. If a notification name
// as already been seen in the chain it ends the list with the notification
// name with a of "..." which indicates that the chain will loop.
func GetNotificationChains(n map[string]*Notification) [][]string {
	chains := [][]string{}
	for _, root := range n {
		chain := []string{}
		seen := make(map[string]bool)
		var walkChain func(next *Notification)
		walkChain = func(next *Notification) {
			if next == nil {
				chains = append(chains, chain)
				return
			}
			if seen[next.Name] {
				chain = append(chain, fmt.Sprintf("...%v", next.Name))
				chains = append(chains, chain)
				return
			}
			chain = append(chain, next.Name)
			seen[next.Name] = true
			walkChain(next.Next)
		}
		walkChain(root)
	}
	return chains
}

// A Lookup is used to return values based on the tags of a response. It
// provides switch/case functionality
type Lookup struct {
	Text    string
	Name    string
	Tags    []string
	Entries []*Entry
	Locator `json:"-"`
}

func (lookup *Lookup) ToExpr() *ExprLookup {
	l := ExprLookup{
		Tags: lookup.Tags,
	}
	for _, entry := range lookup.Entries {
		l.Entries = append(l.Entries, entry.ExprEntry)
	}
	return &l
}

// Entry is an entry in a Lookup.
type Entry struct {
	*ExprEntry
	Def  string
	Name string
}

// Macro provides the ability to reuse partial sections of
// alert definition text. Macros can contain other macros
type Macro struct {
	Text    string
	Pairs   interface{} // this is BAD TODO
	Name    string
	Locator `json:"-"`
}

// Alert stores all information about alerts. All other major
// sections of rule configuration are referenced by alerts including
// Templates, Macros, and Notifications. Alerts hold the expressions
// that determine the Severity of the Alert. There are also flags the
// alter the behavior of the alert and how the expression is evaluated.
// This structure is available to users from templates. Consult documentation
// before making changes
type Alert struct {
	Text string
	Vars
	*Template        `json:"-"`
	Name             string
	Crit             *expr.Expr `json:",omitempty"`
	Warn             *expr.Expr `json:",omitempty"`
	Depends          *expr.Expr `json:",omitempty"`
	Squelch          Squelches  `json:"-"`
	CritNotification *Notifications
	WarnNotification *Notifications
	Unknown          time.Duration
	MaxLogFrequency  time.Duration
	IgnoreUnknown    bool
	UnknownsNormal   bool
	UnjoinedOK       bool `json:",omitempty"`
	Log              bool
	RunEvery         int
	ReturnType       models.FuncType

	TemplateName string   `json:"-"`
	RawSquelch   []string `json:"-"`

	Locator           `json:"-"`
	AlertTemplateKeys map[string]*template.Template `json:"-"`
}

// A Locator stores the information about the location of the rule in the underlying
// rule store
type Locator interface{}

// BulkEditRequest is a collection of BulkEditRequest to be applied sequentially
type BulkEditRequest []EditRequest

// EditRequest is a proposed edit to the config file for sections. The Name is the name of section,
// Type can be "alert", "template", "notification", "lookup", or "macro". The Text should be the full
// text of the definition, including the declaration and brackets (i.e. "alert foo { .. }"). If Delete
// is true then the section will be deleted. In order to rename something, specify the old name in the
// Name field but have the Text definition contain the new name.
type EditRequest struct {
	Name   string
	Type   string
	Text   string
	Delete bool
}

// SaveHook is a function that is passed files as a string (currently the only implementation
// has a single file, so there is no convention for the format of multiple files yet), a user
// a message and vargs. A SaveHook is called when using bosun to save the config. A save is reverted
// when the SaveHook returns an error.
type SaveHook func(files, user, message string, args ...string) error

// MakeSaveCommandHook takes a function based on the command name and will run it on save passing files, user,
// message, args... as arguments to the command. For the SaveHook function that is returned, If the command fails
// to execute or returns a non normal output then an error is returned.
func MakeSaveCommandHook(cmdName string) (f SaveHook, err error) {
	_, err = exec.LookPath(cmdName)
	if err != nil {
		return f, fmt.Errorf("command %v not found, failed to create save hook: %v", cmdName, err)
	}
	f = func(files, user, message string, args ...string) error {
		cArgs := []string{files, user, message}
		cArgs = append(cArgs, args...)
		slog.Infof("executing save hook %v\n", cmdName)
		c := exec.Command(cmdName, cArgs...)
		var cOut bytes.Buffer
		var cErr bytes.Buffer
		c.Stdout = &cOut
		c.Stderr = &cErr
		err := c.Start()
		if err != nil {
			return err
		}
		err = c.Wait()
		if err != nil {
			slog.Warning(cErr.String())
			return fmt.Errorf("%v: %v", err, cErr.String())
		}
		slog.Infof("save hook output: %v\n", cOut.String())
		return nil
	}
	return
}

// GenHash generates a unique hash of a string. It is used so we can compare
// edited text configuration to running text configuration and see if it has
// changed
func GenHash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("%v", h.Sum32())
}
