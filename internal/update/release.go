// Package update checks GitHub releases for newer Aether builds and runs
// self-upgrades through the platform package manager.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// releasesAPI is the upstream Aether release feed.
const releasesAPI = "https://api.github.com/repos/omacom/aether/releases/latest"

// Release is the outcome of an update check.
type Release struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	ReleaseURL      string `json:"releaseURL"`
	UpdateAvailable bool   `json:"updateAvailable"`
	UpdateCommand   string `json:"updateCommand,omitempty"`
}

// versionPattern strips non-numeric prefixes from a tag.
var versionPattern = regexp.MustCompile(`v?([0-9]+(?:\.[0-9]+)+)`)

// httpGet fetches a URL with a bounded body.
func httpGet(url string, limit int64) ([]byte, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return body, resp.StatusCode, err
}

// Check queries GitHub for the latest release and compares against
// currentVersion. Network failures surface as errors; comparison failures
// yield a release with UpdateAvailable=false.
func Check(ctx context.Context, currentVersion string) (*Release, error) {
	if currentVersion == "" || currentVersion == "dev" {
		return &Release{CurrentVersion: currentVersion}, nil
	}

	type ghRelease struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	body, status, err := httpGet(releasesAPI, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("check releases: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("check releases: HTTP %d", status)
	}
	var gh ghRelease
	if err := json.Unmarshal(body, &gh); err != nil {
		return nil, fmt.Errorf("check releases: %w", err)
	}

	latest := versionPattern.FindStringSubmatch(gh.TagName)
	if latest == nil {
		return &Release{CurrentVersion: currentVersion, ReleaseURL: gh.HTMLURL}, nil
	}

	available := isNewer(currentVersion, latest[1])
	release := &Release{
		CurrentVersion:  currentVersion,
		LatestVersion:   latest[1],
		ReleaseURL:      gh.HTMLURL,
		UpdateAvailable: available,
	}
	if available {
		release.UpdateCommand = UpdateCommand()
	}
	return release, nil
}

// isNewer compares dotted version strings.
func isNewer(current, latest string) bool {
	c := splitVersion(current)
	l := splitVersion(latest)
	for i := 0; i < len(l); i++ {
		lv := l[i]
		cv := 0
		if i < len(c) {
			cv = c[i]
		}
		if lv != cv {
			return lv > cv
		}
	}
	return false
}

// splitVersion parses "1.2.3" into integers.
func splitVersion(v string) []int {
	m := versionPattern.FindStringSubmatch(v)
	if m == nil {
		return nil
	}
	parts := strings.Split(m[1], ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out
		}
		out[i] = n
	}
	return out
}
