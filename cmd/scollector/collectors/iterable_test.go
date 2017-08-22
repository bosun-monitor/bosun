package collectors

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bosun.org/cmd/scollector/conf"
)

func mockServerTooSlow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	time.Sleep(2 * time.Second)
	fmt.Fprintln(w, `too late`)
}

// TODO: all tests should have a context given has parameter
// so that we can test ctx.Err() directly
// see
func TestIterableStatSlow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(mockServerTooSlow))
	defer ts.Close()
	iter := conf.Iterable{StatusBaseAddr: ts.URL}
	compKey := IterComp{"API": "api"}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := iterable(ctx, iter, compKey)
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Errorf("should have timed out")
	}
}

func mockServerAllOk(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, allOKAnswer)
}

func TestIterableStatAllOk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(mockServerAllOk))
	defer ts.Close()
	iter := conf.Iterable{StatusBaseAddr: ts.URL}
	compKey := IterComp{
		"Web Application": "webapp",
		"API":             "api",
		"Email Sending":   "email.sending",
		"Email Links":     "email.links",
		"Workflows":       "workflows",
		"Push Sending":    "pushSending",
		// "SMS Sending": nil,
		"System Webhooks":      "systemWebhooks",
		"Analytics Processing": "analyticsProcessing",
		"List Upload":          "listUpload",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	md, err := iterable(ctx, iter, compKey)
	if err != nil {
		t.Errorf("should have no errors: having: %v", err)
	}
	for _, dp := range md {
		if dp.Value != 0 {
			t.Errorf("%v: value should be 0, having: %v", dp.Metric, dp.Value)
		}
	}
}

// This is something the real server could return
var allOKAnswer = `{"page":{"id":"hm1wdv9pcjp9","name":"Iterable","url":"http://status.iterable.com","updated_at":"2017-08-22T00:52:57.454Z"},"components":[{"status":"operational","name":"Web Application","created_at":"2013-08-17T00:37:59.490Z","updated_at":"2017-08-22T00:52:55.578Z","position":1,"description":"The Iterable Web Application","showcase":false,"id":"s38dq3d6bvxh","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"API","created_at":"2013-08-16T21:11:07.992Z","updated_at":"2017-08-16T21:52:34.245Z","position":2,"description":"API Endpoints for Email Campaigns","showcase":false,"id":"st4wbr6s52v6","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"Email Sending","created_at":"2013-08-20T21:00:09.890Z","updated_at":"2017-08-14T16:36:39.660Z","position":4,"description":"Email sending service","showcase":false,"id":"ltndy3zzlys0","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"Email Links","created_at":"2013-08-20T21:00:48.129Z","updated_at":"2016-11-16T02:11:53.077Z","position":5,"description":"Tracking events such as converts, clicks, opens","showcase":false,"id":"wk3bhq37zyxz","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"Workflows","created_at":"2013-08-20T21:05:47.820Z","updated_at":"2017-08-22T00:52:57.354Z","position":6,"description":"Tracking links from emails","showcase":false,"id":"69n5wf13fn2j","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"Push Sending","created_at":"2015-04-15T22:35:36.924Z","updated_at":"2017-07-26T18:53:48.983Z","position":7,"description":null,"showcase":false,"id":"2zpszq733pcd","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"SMS Sending","created_at":"2016-11-16T02:10:36.482Z","updated_at":"2017-07-26T18:53:50.249Z","position":8,"description":null,"showcase":false,"id":"q53wf53znzb6","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"System Webhooks","created_at":"2016-11-16T02:10:51.278Z","updated_at":"2017-07-26T18:53:51.171Z","position":9,"description":null,"showcase":false,"id":"tt0nqf58tx3k","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"Analytics Processing","created_at":"2016-11-16T02:14:03.650Z","updated_at":"2017-07-26T18:53:52.430Z","position":10,"description":null,"showcase":false,"id":"gyxps64956z1","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false},{"status":"operational","name":"List Upload","created_at":"2017-03-30T21:48:11.014Z","updated_at":"2017-06-03T01:54:42.602Z","position":11,"description":null,"showcase":false,"id":"xrwfglh220wc","group_id":null,"page_id":"hm1wdv9pcjp9","group":false,"only_show_if_degraded":false}],"incidents":[],"scheduled_maintenances":[],"status":{"indicator":"none","description":"All Systems Operational"}}`
