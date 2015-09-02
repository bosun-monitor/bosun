package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"bosun.org/_third_party/github.com/google/go-github/github"
)

type myRoundTripper struct {
	accessToken string
}

func (rt myRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", fmt.Sprintf("token %s", rt.accessToken))
	return http.DefaultTransport.RoundTrip(r)
}

var (
	client *github.Client
	number string
	binDir string
	sha    string
)

func init() {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	if accessToken == "" {
		log.Fatal("GITHUB_ACCESS_TOKEN env required")
	}
	if number = os.Getenv("BUILD_NUMBER"); number == "" {
		log.Fatal("BUILD_NUMBER env required")
	}
	if sha = os.Getenv("GIT_SHA"); sha == "" {
		log.Fatal("GIT_SHA env required")
	}
	binDir = os.Getenv("OUTPUTDIR")
	hClient := &http.Client{Transport: myRoundTripper{accessToken}}
	client = github.NewClient(hClient)
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	fmt.Printf("Releasing to github %s.\n", number)

	fmt.Println("Fetching latest release")
	latest, _, err := client.Repositories.GetLatestRelease("bosun-monitor", "bosun")
	checkError(err)

	fmt.Println("Getting all prs since last release to build release notes...")
	opts := &github.PullRequestListOptions{}
	opts.Base = "master"
	opts.Direction = "desc"
	opts.State = "closed"
	opts.Sort = "updated"
	opts.PerPage = 100
	reqs, _, err := client.PullRequests.List("bosun-monitor", "bosun", opts)
	checkError(err)

	// group pr titles if they are prefaced by `cmd/scollector:` or similar.
	groups := make(map[string][]*github.PullRequest)
	for _, pr := range reqs {
		p := pr
		if pr.ClosedAt.Before((*latest.CreatedAt).Time) {
			continue
		}
		if pr.MergedAt == nil {
			continue
		}
		titleParts := strings.SplitN(*pr.Title, ":", 2)
		if len(titleParts) == 1 {
			titleParts = []string{"other", titleParts[0]}
		}
		group := titleParts[0]
		group = strings.Replace(group, "cmd/", "", -1)
		*pr.Title = titleParts[1]
		groups[group] = append(groups[group], &p)
	}

	body := ""
	for key, prs := range groups {
		body += fmt.Sprintf("\n### %s: ###\n", key)
		for _, pr := range prs {
			body += fmt.Sprintf("  - %s [#%d](%s)\n", *pr.Title, *pr.Number, *pr.HTMLURL)
		}
	}

	fmt.Println("Creating the release...")
	release := &github.RepositoryRelease{}
	release.TagName = &number
	release.TargetCommitish = &sha
	release.Name = &number
	isDraft := true
	release.Draft = &isDraft
	release.Body = &body
	release, _, err = client.Repositories.CreateRelease("bosun-monitor", "bosun", release)
	checkError(err)

	fmt.Println("Uploading artifacts...")
	files, err := ioutil.ReadDir(binDir)
	checkError(err)
	wg := sync.WaitGroup{}
	for _, file := range files {
		if file.IsDir() {
			return
		}
		wg.Add(1)
		filename := file.Name()
		go func() {
			uploadArtifact(filename, *release.ID)
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Printf("Done. %s\n", *release.HTMLURL)
}

func uploadArtifact(filename string, id int) {
	fmt.Println("\t", filename)
	f, err := os.Open(filepath.Join(binDir, filename))
	checkError(err)
	defer f.Close()
	opts := &github.UploadOptions{}
	opts.Name = filename
	_, _, err = client.Repositories.UploadReleaseAsset("bosun-monitor", "bosun", id, opts, f)
	checkError(err)
}
