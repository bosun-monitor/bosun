package sched

import (
	"strings"
	"testing"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/models"
)

var tstSimple = `
notification pduty {
	post = https://example.com/submit
	template = postData
}
notification e {
	email = test2@example.com
	template = body
}
template pduty {
	postData = '{"some":"json", "token":"{{.Alert.Vars.x}}","title":{{.Subject}}}'
}
template t {
	subject = "aaaaa"
	body = "some bad stuff happened"
	inherit = pduty
}
alert a {
	$x = "foo"
	template = t
	crit = 1
    critNotification = pduty,e
}
`

type templateTest struct {
	Desc    string
	Input   string
	Subject string
	Body    string
	Others  map[string]string
}

// all will have a standard alert added to config, that references t.
// Use single quite instead of backticks to define template string keys
var templateTests = []*templateTest{
	{
		Desc: "Simple - Nothing Special",
		Input: `
template t {
	subject = 'aaaaa'
	body = 'bbbbb'
}
`,
		Subject: "aaaaa",
		Body:    "bbbbb",
	},

	{
		Desc: "Inherit",
		Input: `
template t2{
	body = 'bbbbb'
}
template t {
	subject = 'aaaaa'
	inherit = t2
}
`,
		Subject: "aaaaa",
		Body:    "bbbbb",
	},

	{
		Desc: "Custom Key",
		Input: `
template t {
	subject = 'aaaaa'
	body = 'b'
	pduty = '{"json": true}'
}
`,
		Subject: "aaaaa",
		Body:    "b",
		Others:  map[string]string{"pduty": `{"json": true}`},
	},

	{
		Desc: "Inherited Custom Key",
		Input: `
template pduty {
	pduty = '[42]'
}

template t {
	subject = 'aaaaa'
	body = 'b'
	inherit = pduty
}
`,
		Subject: "aaaaa",
		Body:    "b",
		Others:  map[string]string{"pduty": `[42]`},
	},

	{
		Desc: "Inherited Tpl uses alert var",
		Input: `
template pduty {
	pduty = 'http://ex.com?alert={{.Alert.Vars.x}}'
}
template t {
	subject = 'aaaaa'
	body = 'b'
	inherit = pduty
}
`,
		Subject: "aaaaa",
		Body:    "b",
		Others:  map[string]string{"pduty": `http://ex.com?alert=foo`},
	},

	{
		Desc: "Inherited Tpl uses subject",
		Input: `
template pduty {
	pduty = 'http://ex.com?alert={{.Subject}}'
}
template t {
	subject = 'abcde'
	body = 'b'
	inherit = pduty
}
`,
		Subject: "abcde",
		Body:    "b",
		Others:  map[string]string{"pduty": `http://ex.com?alert=abcde`},
	},

	{
		Desc: "Execute Another Key",
		Input: `
template t {
	subject = 'abcde'
	body = 'b'
	pduty = 'http://ex.com?alert={{.Template "foo"}}'
	foo = 'foo'
}
`,
		Subject: "abcde",
		Body:    "b",
		Others:  map[string]string{"pduty": `http://ex.com?alert=foo`, "foo": "foo"},
	},

	{
		Desc: "Get/Set for Another Key",
		Input: `
template pduty {
	
}
template t {
	subject = 'abcde'
	body = 'b'
	pduty = '{{.Set "a" true}}foo={{.Template "foo"}}'
	pduty2 = '{{.Set "a" false}}foo={{.Template "foo"}}'
	foo = '{{if .Get "a"}}a{{else}}b{{end}}'
}
`,
		Subject: "abcde",
		Body:    "b",
		Others:  map[string]string{"pduty": `foo=a`, "pduty2": `foo=b`, "foo": "b"},
	},
}

func TestTemplateRendering(t *testing.T) {
	const alertBlock = `
alert a {
	$x = foo
	template = t
	crit = 1
}`
	for _, tst := range templateTests {
		t.Run(tst.Desc, func(t *testing.T) {
			tst.Input = strings.Replace(tst.Input, "'", "`", -1)
			c, err := rule.NewConf("test", conf.EnabledBackends{}, nil, tst.Input+alertBlock)
			if err != nil {
				t.Fatal(err)
			}
			sched := new(Schedule)
			sched.RuleConf = c
			st := &models.IncidentState{}
			rts, err := sched.ExecuteAll(nil, c.GetAlert("a"), st)
			if err != nil {
				t.Fatal(err)
			}
			if rts == nil {
				t.Fatal("Should not get nil result")
			}
			if st.Subject != tst.Subject {
				t.Errorf("Rendered subject does not match. Got %s, should get %s.", st.Subject, tst.Subject)
			}
			if rts.Body != tst.Body {
				t.Errorf("Rendered body does not match. Got %s, should get %s.", rts.Body, tst.Body)
			}
			for k, v := range tst.Others {
				found, ok := rts.CustomTemplates[k]
				if !ok {
					t.Errorf("Did not find rendered version of %s", k)
					continue
				}
				if found != v {
					t.Errorf("Rendered %s does not match. Got %s, should get %s.", k, found, v)
				}
			}
		})
	}

}
