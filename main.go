package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
	"log"
	"os"
)

func main() {

	u := os.Getenv("GITHUB_USER")
	fmt.Println(u)
	token, found := os.LookupEnv("GITHUB_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	ctx := context.Background()

	// Create the http client.
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	gh := github.NewClient(tc)

	rg, _, err := gh.Actions.CreateOrganizationRunnerGroup(ctx, "gh-walker", github.CreateRunnerGroupRequest{
		Name:                     github.String("self-hosted"),
		Visibility:               github.String("private"),
		SelectedRepositoryIDs:    nil,
		Runners:                  nil,
		AllowsPublicRepositories: github.Bool(true), // false
	})
	if err != nil {
		panic(err)
	}
	printJSON(rg)

	// gh.Actions.CreateOrganizationRegistrationToken()
}

func printJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
