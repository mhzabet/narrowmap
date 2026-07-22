package app

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeArchiveBackend struct {
	inventory map[string][]string
	versions  map[string][]string
	responses map[string]archiveResponse
	calls     []string
}

func (backend *fakeArchiveBackend) Inventory(_ context.Context, target string, noSubs bool) ([]string, error) {
	backend.calls = append(backend.calls, "inventory "+target)
	return backend.inventory[target], nil
}

func (backend *fakeArchiveBackend) Versions(_ context.Context, original string) ([]string, error) {
	backend.calls = append(backend.calls, "versions "+original)
	return backend.versions[original], nil
}

func (backend *fakeArchiveBackend) Fetch(_ context.Context, replayURL string, _ int64) (archiveResponse, error) {
	backend.calls = append(backend.calls, "fetch "+replayURL)
	return backend.responses[replayURL], nil
}

func (backend *fakeArchiveBackend) Close() {}

func TestRobofinderWorkflowUsesDistinctArchivedRobotsContexts(t *testing.T) {
	const original = "https://example.test/robots.txt"
	firstReplay := "https://web.archive.org/web/20200101000000id_/" + original
	secondReplay := "https://web.archive.org/web/20220101000000id_/" + original
	backend := &fakeArchiveBackend{
		inventory: map[string][]string{
			"example.test": {original, "https://example.test/static/app.js"},
		},
		versions: map[string][]string{
			original: {
				strings.Replace(firstReplay, "id_", "if_", 1),
				strings.Replace(secondReplay, "id_", "if_", 1),
			},
		},
		responses: map[string]archiveResponse{
			firstReplay:  {status: 200, contentType: "text/plain", body: []byte("Disallow: /old-admin\n")},
			secondReplay: {status: 200, contentType: "text/plain", body: []byte("Disallow: /old-admin\nAllow: /api/internal\n")},
		},
	}
	cfg := archiveTestConfig()
	var output bytes.Buffer
	findings := newCollector(&output, false)
	warnings := 0
	err := runArchiveWorkflow(
		context.Background(), cfg, archiveRobofinder, []string{"example.test"}, backend,
		findings, newProgress(io.Discard, false), func(string, error) { warnings++ },
	)
	if err != nil {
		t.Fatal(err)
	}
	if warnings != 0 {
		t.Fatalf("unexpected warnings: %d", warnings)
	}
	want := []string{"https://example.test/api/internal", "https://example.test/old-admin"}
	if !reflect.DeepEqual(findings.Sorted(), want) {
		t.Fatalf("got %v, want %v", findings.Sorted(), want)
	}
	for _, replay := range []string{firstReplay, secondReplay} {
		if !containsString(backend.calls, "fetch "+replay) {
			t.Errorf("raw archived replay was not fetched: %s", replay)
		}
	}
}

func TestOJSWorkflowFetchesContextInMemoryAndSamplesAcrossVersions(t *testing.T) {
	const original = "https://cdn.example.test/common.js?v=1"
	oldReplay := "https://web.archive.org/web/20190101000000id_/" + original
	newReplay := "https://web.archive.org/web/20240101000000id_/" + original
	backend := &fakeArchiveBackend{
		inventory: map[string][]string{
			"example.test": {original, "https://cdn.example.test/logo.svg"},
		},
		versions: map[string][]string{
			original: {oldReplay, newReplay},
		},
		responses: map[string]archiveResponse{
			newReplay: {status: 200, contentType: "application/javascript", body: []byte(`fetch('/api/v3/users?token=secret')`)},
		},
	}
	cfg := archiveTestConfig()
	cfg.archiveMaxVersions = 1
	findings := newCollector(io.Discard, false)
	err := runArchiveWorkflow(
		context.Background(), cfg, archiveOJS, []string{"example.test"}, backend,
		findings, newProgress(io.Discard, false), func(source string, err error) {
			t.Fatalf("unexpected warning for %s: %v", source, err)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://cdn.example.test/api/v3/users?token="}
	if !reflect.DeepEqual(findings.Sorted(), want) {
		t.Fatalf("got %v, want %v", findings.Sorted(), want)
	}
	if containsString(backend.calls, "fetch "+oldReplay) {
		t.Fatal("maximum version limit did not select only the latest context")
	}
	if !containsString(backend.calls, "fetch "+newReplay) {
		t.Fatal("latest archived JavaScript context was not requested")
	}
}

func TestSelectArchiveVersionsSamplesFirstMiddleAndLatest(t *testing.T) {
	values := []string{
		"https://web.archive.org/web/20200101000000if_/https://example.test/app.js",
		"https://web.archive.org/web/20210101000000if_/https://example.test/app.js",
		"https://web.archive.org/web/20220101000000if_/https://example.test/app.js",
		"https://web.archive.org/web/20230101000000if_/https://example.test/app.js",
		"https://web.archive.org/web/20240101000000if_/https://example.test/app.js",
	}
	selected := selectArchiveVersions(values, 3)
	want := []string{
		"https://web.archive.org/web/20200101000000id_/https://example.test/app.js",
		"https://web.archive.org/web/20220101000000id_/https://example.test/app.js",
		"https://web.archive.org/web/20240101000000id_/https://example.test/app.js",
	}
	if !reflect.DeepEqual(selected, want) {
		t.Fatalf("got %v, want %v", selected, want)
	}
}

func TestArchiveConfigEnforcesSlowCredentialSafeModes(t *testing.T) {
	for _, test := range []struct {
		args    []string
		message string
	}{
		{[]string{"--robofinder", "example.test", "--archive-delay", "100ms"}, "at least 500ms"},
		{[]string{"--ojs", "example.test", "-H", "Cookie: secret"}, "must not be sent"},
		{[]string{"--input-file", "app.js", "--archive-no-subs"}, "require --robofinder or --ojs"},
	} {
		_, err := parseConfig(test.args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("%v: got %v, want error containing %q", test.args, err, test.message)
		}
	}

	cfg, err := parseConfig([]string{"--oJs", "example.test"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ojs != "example.test" || cfg.archiveDelay != 2*time.Second {
		t.Fatalf("unexpected oJs config: %+v", cfg)
	}
}

func TestLoadArchiveTargetsFromStdinNormalizesAndDeduplicates(t *testing.T) {
	targets, err := loadArchiveTargets("-", strings.NewReader("*.Example.test\nhttps://example.test/path\napi.example.test\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.test", "example.test"}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("got %v, want %v", targets, want)
	}
}

func archiveTestConfig() config {
	return config{
		archiveDelay:       0,
		archiveRetries:     0,
		archiveMaxVersions: 0,
		maxBody:            1024 * 1024,
	}
}
