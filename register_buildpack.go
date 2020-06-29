package pack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"text/template"

	"github.com/buildpacks/pack/internal/buildpackage"
	"github.com/buildpacks/pack/internal/dist"
	"github.com/buildpacks/pack/internal/registry"
)

type RegisterBuildpackOptions struct {
	ImageName string
	Type      string
	URL       string
}

func (c *Client) RegisterBuildpack(ctx context.Context, opts RegisterBuildpackOptions) error {
	appImage, err := c.imageFetcher.Fetch(ctx, opts.ImageName, false, true)
	if err != nil {
		return err
	}

	label, err := appImage.Label(buildpackage.MetadataLabel)
	if err != nil {
		return err
	}

	c.logger.Debugf("Found image label %s: %s", buildpackage.MetadataLabel, label)
	var buildpackInfo dist.BuildpackInfo
	if err = json.Unmarshal([]byte(label), &buildpackInfo); err != nil {
		return err
	}

	namespace, name, err := parseID(buildpackInfo.ID)
	if err != nil {
		return err
	}

	id, err := appImage.Identifier()
	if err != nil {
		return err
	}

	buildpack := registry.Buildpack{
		Namespace: namespace,
		Name:      name,
		Version:   buildpackInfo.Version,
		Address:   id.String(),
		Yanked:    false,
	}

	issueTitle, err := createGithubIssueTitle(buildpack)
	if err != nil {
		return err
	}

	issueBody, err := createGithubIssueBody(buildpack)
	if err != nil {
		return err
	}

	issueURL, err := parseURL(opts.URL)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("title", issueTitle)
	params.Add("body", issueBody)
	issueURL.RawQuery = params.Encode()

	c.logger.Debugf("Open URL in browser: %s", issueURL)
	return openBrowser(issueURL.String())
}

var execCommand = exec.Command

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = execCommand("xdg-open", url).Start()
	case "windows":
		err = execCommand("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = execCommand("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

func parseID(id string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return "", "", errors.New("invalid id: does not contain a namespace")
	} else if len(parts) > 2 {
		return "", "", errors.New("invalid id: contains unexpected characters")
	}

	return parts[0], parts[1], nil
}

func parseURL(githubURL string) (*url.URL, error) {
	if githubURL == "" {
		return nil, errors.New("missing github URL")
	}
	return url.Parse(fmt.Sprintf("%s/issues/new", strings.TrimSuffix(githubURL, "/")))
}

func createGithubIssueBody(buildpack registry.Buildpack) (string, error) {
	ut, err := template.New("buildpack").Parse(registry.GithubIssueBodyTemplate)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = ut.Execute(&tpl, buildpack)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}

func createGithubIssueTitle(buildpack registry.Buildpack) (string, error) {
	ut, err := template.New("buildpack").Parse(registry.GithubIssueTitleTemplate)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = ut.Execute(&tpl, buildpack)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}
