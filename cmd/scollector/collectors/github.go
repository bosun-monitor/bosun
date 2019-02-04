package collectors

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/google/go-github/github"
)

func init() {
	RegisterInit(startGithubCollectors)
}

type githubRoundTripper struct {
	accessToken string
}

func (rt githubRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", fmt.Sprintf("token %s", rt.accessToken))
	return http.DefaultTransport.RoundTrip(r)
}

func startGithubCollectors(c *conf.Conf) {
	for _, gh := range c.Github {
		client := github.NewClient(&http.Client{Transport: githubRoundTripper{gh.Token}})
		split := strings.Split(gh.Repo, "/")
		if len(split) != 2 {
			slog.Fatal("Repo must have two parts (owner/repo)")
		}
		owner, repo := split[0], split[1]
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return githubCollect(client, owner, repo)
			},
			name:     fmt.Sprintf("github-%s", gh.Repo),
			Interval: 10 * time.Minute, //10 minutes to respect api limits
		})
	}
}

const (
	descGithubOpenIssues = "Number of currently open issues"
	descGithubOpenPrs    = "Number of currently open pull requests"
)

func githubCollect(client *github.Client, owner, repo string) (o opentsdb.MultiDataPoint, e error) {
	var md opentsdb.MultiDataPoint
	opts := &github.IssueListByRepoOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}
	openIssueCount := 0
	openPrCount := 0
	issueLabelCounts := make(map[string]int)
	prLabelCounts := make(map[string]int)
	for {
		issues, resp, err := client.Issues.ListByRepo(owner, repo, opts)
		if err != nil {
			return md, err
		}
		for _, i := range issues {
			if i.PullRequestLinks != nil {
				openPrCount++
				for _, label := range i.Labels {
					prLabelCounts[*label.Name] = prLabelCounts[*label.Name] + 1
				}
			} else {
				openIssueCount++
				for _, label := range i.Labels {
					issueLabelCounts[*label.Name] = issueLabelCounts[*label.Name] + 1
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	Add(&md, "github.open_issues", openIssueCount, opentsdb.TagSet{"repo": repo}, metadata.Gauge, metadata.Count, descGithubOpenIssues)
	for name, count := range issueLabelCounts {
		Add(&md, "github.open_issues_label", count, opentsdb.TagSet{"repo": repo, "label": name}, metadata.Gauge, metadata.Count, descGithubOpenIssues)
	}
	Add(&md, "github.open_prs", openPrCount, opentsdb.TagSet{"repo": repo}, metadata.Gauge, metadata.Count, descGithubOpenPrs)
	for name, count := range prLabelCounts {
		Add(&md, "github.open_prs_label", count, opentsdb.TagSet{"repo": repo, "label": name}, metadata.Gauge, metadata.Count, descGithubOpenPrs)
	}
	return md, nil
}
