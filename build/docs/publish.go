package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/github"
)

type myRoundTripper struct {
	accessToken string
}

func (rt myRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", fmt.Sprintf("token %s", rt.accessToken))
	return http.DefaultTransport.RoundTrip(r)
}

var (
	client   *github.Client
	msg      = "syncing to gh-pages"
	gh_pages = "heads/gh-pages"
	force    = flag.Bool("f", false, "Push a new commit, even if code is identical. Forces gh-pages rebuild.")
)

func init() {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	if accessToken == "" {
		log.Fatal("GITHUB_ACCESS_TOKEN env required")
	}
	hClient := &http.Client{Transport: myRoundTripper{accessToken}}
	client = github.NewClient(hClient)
}

const (
	o = "bosun-monitor"
	r = "bosun"
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Sync docs folder to gh-pages branch by creating a new commit with the appropriate tree.
func main() {
	flag.Parse()

	// Fetch current master
	branch, _, err := client.Repositories.GetBranch(o, r, "master")
	checkError(err)
	fmt.Println("Master commit hash:", *branch.Commit.SHA)

	// Get tip commit hash
	masterTip, _, err := client.Git.GetCommit(o, r, *branch.Commit.SHA)
	checkError(err)
	fmt.Println("Commit tree:", *masterTip.Tree.SHA)

	// Find tree representing docs
	tree, _, err := client.Git.GetTree(o, r, *masterTip.Tree.SHA, false)
	checkError(err)
	docsSha := ""
	for _, entry := range tree.Entries {
		if *entry.Path == "docs" && *entry.Type == "tree" {
			docsSha = *entry.SHA
			break
		}
	}
	fmt.Println("docs tree:", docsSha)

	// Get gh-pages branch
	ghBranch, _, err := client.Repositories.GetBranch(o, r, "gh-pages")
	checkError(err)
	fmt.Println("gh-pages commit hash:", *ghBranch.Commit.SHA)

	// Check root tree for differences
	ghTip, _, err := client.Git.GetCommit(o, r, *ghBranch.Commit.SHA)
	checkError(err)
	fmt.Println("gh-pages tree:", *ghTip.Tree.SHA)
	if *ghTip.Tree.SHA == docsSha && !*force {
		fmt.Println("Nothing to do.")
		return
	}

	// form new commit and push it
	newCommit := &github.Commit{}
	newCommit.Message = &msg
	newCommit.Tree = &github.Tree{SHA: &docsSha}
	newCommit.Parents = []github.Commit{*ghTip}
	newCommit, _, err = client.Git.CreateCommit(o, r, newCommit)
	checkError(err)
	fmt.Println("New docs commit:", *newCommit.SHA)

	// Update gh-pages to point at new commit
	ref := &github.Reference{Ref: &gh_pages}
	ref.Object = &github.GitObject{SHA: newCommit.SHA}
	ref, _, err = client.Git.UpdateRef(o, r, ref, true)
	checkError(err)
	fmt.Println(ref)
}
