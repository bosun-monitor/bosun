package conf

import (
	"regexp"
	"strings"
	"testing"
)

const dependencyTestConfig = `
	tsdbHost=localhost:4242
	macro host_based{
		depends = alert("scollector.down","warn")
	}
	alert ping.host{
		$q = max(rename(q("sum:bosun.ping.timeout{dst_host=*,host=*}", "1m", ""), "host=source,dst_host=host"))
		warn = $q
	}
	alert scollector.down{
		depends = alert("ping.host", "warn")
		$a = avg(q("avg:os.cpu{host=*}", "1m", "")) < -100
		warn = $a
	}
	alert os.cpu{
		macro = host_based
		warn = avg(q("avg:os.cpu{host=*}", "1m", "")) > 50
	}
`

func TestConfDependencies(t *testing.T) {
	c, err := New("test.conf", dependencyTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	templates, err := c.AlertTemplateStrings()
	if err != nil {
		t.Fatal(err)
	}
	expected := "ping.host,scollector.down,host_based,os.cpu"
	assertTemplateSequenceEqual(t, templates, "os.cpu", expected)

	expected = "ping.host"
	assertTemplateSequenceEqual(t, templates, "ping.host", expected)
}

func assertTemplateSequenceEqual(t *testing.T, templates *AlertTemplateStrings, alert, expected string) {
	result := templateToSequence(templates.Alerts[alert])
	if result != expected {
		t.Fatalf("Bad template sequence. Expected: %s. Got: %s.", expected, result)
	}
}

// Returns general order of components in template string. Comma delimited list of names, from top to bottom.
func templateToSequence(template string) string {
	regex := regexp.MustCompile(`(alert|macro|notification|lookup) ([a-z0-9\._]+)(\s*)?\{`)
	matches := regex.FindAllStringSubmatch(template, -1)
	names := []string{}
	for _, match := range matches {
		names = append(names, match[2])
	}
	return strings.Join(names, ",")
}
