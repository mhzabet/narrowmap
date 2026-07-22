package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunInputFolderProducesSortedUniqueOutput(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "page.html"), `
		<input id="login_email" name="email">
		<a href="/next?redirect_to=dashboard"></a>
		<script>const pageConfig = { user_id: 1 };</script>
	`)
	writeTestFile(t, filepath.Join(root, "data.json"), `{"account_id": 1, "nested": {"email": "a@example.test"}}`)
	writeTestFile(t, filepath.Join(root, "ignored.txt"), `should_not_be_seen=1`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"--input-folder", root, "-v-param"},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}

	lines := nonemptyLines(stdout.String())
	if !sort.StringsAreSorted(lines) {
		t.Fatalf("output is not sorted: %v", lines)
	}
	for _, expected := range []string{
		"account_id", "email", "login_email", "nested", "redirect_to", "user_id",
	} {
		if !containsString(lines, expected) {
			t.Errorf("missing %q in %v", expected, lines)
		}
	}
	if strings.Contains(stdout.String(), "should_not_be_seen") {
		t.Fatal("unsupported .txt file was analyzed")
	}
	if !strings.Contains(stderr.String(), "[+] analyzing 2 local files") {
		t.Fatalf("expected staged progress, got %q", stderr.String())
	}
}

func TestRunInputLinksFetchesHTMLAndSameOriginJavaScript(t *testing.T) {
	var sawHeader atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Narrow-Test") == "yes" {
			sawHeader.Store(true)
		}
		switch request.URL.Path {
		case "/page":
			writer.Header().Add("Set-Cookie", "session_id=abc; Path=/; HttpOnly")
			writer.Header().Set("Content-Type", "text/html")
			fmt.Fprint(writer, `
				<input name="search_term" id="search-box">
				<a href="/next?redirect_to=home"></a>
				<script src="/app.js?build_id=7"></script>
			`)
		case "/app.js":
			writer.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(writer, `const remoteConfig = { api_token: tokenValue, object_id: 7 };`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	linksPath := filepath.Join(t.TempDir(), "links.txt")
	writeTestFile(t, linksPath, server.URL+"/page?seed_id=1\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(
		context.Background(),
		[]string{
			"--input-links", linksPath,
			"-v-param",
			"--silent",
			"--concurrency", "2",
			"--delay", "0",
			"-H", "X-Narrow-Test: yes",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !sawHeader.Load() {
		t.Fatal("custom request header was not sent")
	}

	lines := nonemptyLines(stdout.String())
	for _, expected := range []string{
		"api_token", "build_id", "object_id", "redirect_to",
		"search-box", "search_term", "seed_id", "session_id",
	} {
		if !containsString(lines, expected) {
			t.Errorf("missing %q in %v", expected, lines)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("silent mode should not print progress: %q", stderr.String())
	}
}

func TestRunInputURLCanSkipSameOriginJavaScript(t *testing.T) {
	var scriptRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/app.js" {
			scriptRequested.Store(true)
			writer.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(writer, `const api_token = "redacted";`)
			return
		}
		writer.Header().Set("Content-Type", "text/html")
		fmt.Fprint(writer, `<input name="email"><script src="/app.js"></script>`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--input-url", server.URL, "--silent", "--delay", "0", "--no-same-origin-js"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if scriptRequested.Load() {
		t.Fatal("same-origin JavaScript was fetched despite --no-same-origin-js")
	}
	if !containsString(nonemptyLines(stdout.String()), "email") {
		t.Fatalf("expected HTML parameter in %q", stdout.String())
	}
	if containsString(nonemptyLines(stdout.String()), "api_token") {
		t.Fatalf("unexpected JavaScript parameter in %q", stdout.String())
	}
}

func TestRunInputFileFromStdin(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--input-file", "--silent"},
		strings.NewReader(`<div id="layout-wrapper"></div><input id="email-field" name="email">`),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}

	lines := nonemptyLines(stdout.String())
	for _, expected := range []string{"email", "email-field"} {
		if !containsString(lines, expected) {
			t.Errorf("missing stdin file candidate %q in %v", expected, lines)
		}
	}
	if containsString(lines, "layout-wrapper") {
		t.Fatalf("layout id should be filtered: %v", lines)
	}
}

func TestRunInputURLFromStdin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"user_id": 1}`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--input-url", "--silent", "--delay", "0"},
		strings.NewReader(server.URL+"?request_id=7\n"),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	lines := nonemptyLines(stdout.String())
	for _, expected := range []string{"request_id", "user_id"} {
		if !containsString(lines, expected) {
			t.Errorf("missing stdin URL candidate %q in %v", expected, lines)
		}
	}
}

func TestRunParamgenFileProducesSortedUsefulOutput(t *testing.T) {
	root := t.TempDir()
	seedsPath := filepath.Join(root, "params.txt")
	prefixesPath := filepath.Join(root, "prefixes.txt")
	suffixesPath := filepath.Join(root, "suffixes.txt")
	outputPath := filepath.Join(root, "generated.txt")
	writeTestFile(t, seedsPath, "user_id\naccountId\nyahoo_home_ui\nuser_id\nuseEffect\n")
	writeTestFile(t, prefixesPath, "internal\n")
	writeTestFile(t, suffixesPath, "checksum\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{
			"--paramgen", seedsPath,
			"--paramgen-prefixes", prefixesPath,
			"--paramgen-suffixes", suffixesPath,
			"-o", outputPath,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}

	lines := nonemptyLines(stdout.String())
	if !sort.StringsAreSorted(lines) {
		t.Fatalf("paramgen output is not sorted: %v", lines)
	}
	for _, expected := range []string{
		"user_id", "accountId", "user_uuid", "accountUuid",
		"internal_user_id", "user_checksum", "yahoo_home_redirect",
	} {
		if !containsString(lines, expected) {
			t.Errorf("missing generated parameter %q in %v", expected, lines)
		}
	}
	if containsString(lines, "useEffect") {
		t.Fatal("framework runtime noise was retained")
	}

	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(written) {
		t.Fatal("-o output differs from sorted stdout output")
	}
	for _, stage := range []string{
		"[+] target-specific parameter generation",
		"[+] accepted 3 seed parameters (1 invalid, 1 duplicate)",
	} {
		if !strings.Contains(stderr.String(), stage) {
			t.Errorf("missing progress stage %q in %q", stage, stderr.String())
		}
	}
}

func TestRunParamgenFromStdinIsSilentAndSorted(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--paramgen", "--silent"},
		strings.NewReader("status\nuser_id\naccount_id\n"),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}

	lines := nonemptyLines(stdout.String())
	if !sort.StringsAreSorted(lines) {
		t.Fatalf("silent paramgen output is not deterministic: %v", lines)
	}
	if containsString(lines, "status_id") {
		t.Fatal("generic observed suffix was applied to status")
	}
	if !containsString(lines, "payment_status") {
		t.Fatalf("missing semantic status candidate in %v", lines)
	}
	if stderr.Len() != 0 {
		t.Fatalf("silent paramgen should not print progress: %q", stderr.String())
	}
}

func TestRunParamgenRejectsInputModeConflicts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--paramgen", "params.txt", "--input-file", "page.html"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil || !strings.Contains(err.Error(), "choose exactly one") {
		t.Fatalf("expected an input-mode conflict, got %v", err)
	}
}

func TestNormalizeHTTPURLAddsHTTPS(t *testing.T) {
	actual, err := normalizeHTTPURL("target.example/path?user_id=1")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "https://target.example/path?user_id=1" {
		t.Fatalf("got %q", actual)
	}
}

func TestLoadLinksFromStdinAcceptsBareHosts(t *testing.T) {
	links, err := loadLinks("-", strings.NewReader("target.example\nhttps://other.example/path\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://target.example", "https://other.example/path"}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("got %v, want %v", links, want)
	}
}

func TestAllParamsRestoresLowSignalNames(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithInput(
		context.Background(),
		[]string{"--input-file", "-v-param", "--all-params", "--silent"},
		strings.NewReader(`<div id="layout-wrapper" name="layoutName"></div><script>const pageConfig = {};</script>`),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	lines := nonemptyLines(stdout.String())
	for _, expected := range []string{"layoutName", "pageConfig"} {
		if !containsString(lines, expected) {
			t.Errorf("--all-params should retain %q in %v", expected, lines)
		}
	}
	if containsString(lines, "layout-wrapper") {
		t.Fatalf("--all-params restored a non-input id: %v", lines)
	}
}

func TestParseByteSize(t *testing.T) {
	tests := map[string]int64{
		"10MB": 10 * 1024 * 1024,
		"8KB":  8 * 1024,
		"512B": 512,
		"42":   42,
	}
	for input, expected := range tests {
		actual, err := parseByteSize(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if actual != expected {
			t.Errorf("%s: got %d, want %d", input, actual, expected)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func nonemptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
