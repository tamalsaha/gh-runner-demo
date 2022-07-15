package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/linode/linodego"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

func main() {
	token := os.Getenv("LINODE_CLI_TOKEN")
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	c := linodego.NewClient(oauth2Client)

	scriptID, err := getStartupScriptID(&c)
	if err != nil {
		panic(err)
	}
	fmt.Println(scriptID)

	// scriptID := 1018111
}

func getStartupScriptID(c *linodego.Client) (int, error) {
	scriptName := "gh-runner"
	filter := fmt.Sprintf(`{"label" : "%v"}`, scriptName)
	listOpts := &linodego.ListOptions{PageOptions: nil, Filter: filter}

	scripts, err := c.ListStackscripts(context.Background(), listOpts)
	if err != nil {
		return 0, err
	}

	if len(scripts) > 1 {
		return 0, errors.Errorf("multiple stackscript found with label %v", scriptName)
	} else if len(scripts) == 0 {
		return 0, errors.Errorf("no stackscript found with label %v", scriptName)
	}
	return scripts[0].ID, nil
}
