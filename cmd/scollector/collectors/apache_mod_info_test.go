package collectors

import (
	"strings"
	"testing"

	"bosun.org/_third_party/golang.org/x/net/html"
)

const (
	apacheModInfoServerHTML = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"
"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
  <title>Server Information</title>
</head>
<body><h1 style="text-align: center">Apache Server Information</h1>
<h2><a name="server">Server Settings</a></h2><dl><dt><strong>Server Version:</strong> <font size="+1"><tt>Apache/2.2.15 (Unix) DAV/2 PHP/5.3.3</tt></font></dt>
<dt><strong>Server Built:</strong> <font size="+1"><tt>May 13 2013 22:11:16</tt></font></dt>
<dt><strong>Server loaded APR Version:</strong> <tt>1.3.9</tt></dt>
<dt><strong>Compiled with APR Version:</strong> <tt>1.3.9</tt></dt>
<dt><strong>Server loaded APU Version:</strong> <tt>1.3.9</tt></dt>
<dt><strong>Compiled with APU Version:</strong> <tt>1.3.9</tt></dt>
<dt><strong>Module Magic Number:</strong> <tt>20051115:25</tt></dt>
<dt><strong>Hostname/port:</strong> <tt>127.0.0.1:80</tt></dt>
<dt><strong>Timeouts:</strong> <tt>connection: 60 &nbsp;&nbsp; keep-alive: 15</tt></dt><dt><strong>MPM Name:</strong> <tt>Prefork</tt></dt>
<dt><strong>MPM Information:</strong> <tt>Max Daemons: 64 Threaded: no Forked: yes</tt></dt>
<dt><strong>Server Architecture:</strong> <tt>64-bit</tt></dt>
<dt><strong>Server Root:</strong> <tt>/etc/httpd</tt></dt>
<dt><strong>Config File:</strong> <tt>/etc/httpd/conf/httpd.conf</tt></dt>
<dt><strong>Server Built With:</strong>
<tt style="white-space: pre;">
 -D APACHE_MPM_DIR="server/mpm/prefork"
 -D APR_HAS_SENDFILE
 -D APR_HAS_MMAP
 -D APR_HAVE_IPV6 (IPv4-mapped addresses enabled)
 -D APR_USE_SYSVSEM_SERIALIZE
 -D SINGLE_LISTEN_UNSERIALIZED_ACCEPT
 -D APR_HAS_OTHER_CHILD
 -D AP_HAVE_RELIABLE_PIPED_LOGS
 -D HTTPD_ROOT="/etc/httpd"
 -D SUEXEC_BIN="/usr/sbin/suexec"
 -D DEFAULT_ERRORLOG="logs/error_log"
 -D AP_TYPES_CONFIG_FILE="conf/mime.types"
 -D SERVER_CONFIG_FILE="conf/httpd.conf"
</tt></dt>
</dl><hr /><address>Apache/2.2.15 (CentOS) Server at 127.0.0.1 Port 80</address>
</body></html>
`
)

func TestApacheModInfoEmpty(t *testing.T) {

	n, err := html.Parse(strings.NewReader(""))
	if err != nil {
		t.Errorf("unable to parse ?server status page")
	}

	c, k, err := extractTimeouts(n)
	if c != 0 || k != 0 || err == nil {
		t.Errorf("Expected 0,0,ERROR got %v,%v,%v", c, k, err)
	}

}

func TestApacheModInfoServerSimple(t *testing.T) {
	n, err := html.Parse(strings.NewReader(apacheModInfoServerHTML))
	if err != nil {
		t.Errorf("unable to parse ?server status page")
	}

	c, k, err := extractTimeouts(n)
	if c != 60 || k != 15 || err != nil {
		t.Errorf("Expected 60,15,<nil> got %v,%v,%v", c, k, err)
	}
}
