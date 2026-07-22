package extract

import (
	"reflect"
	"sort"
	"testing"
)

func TestJavaScriptEndpointsExtractsRoutesWithoutStaticAssetNoise(t *testing.T) {
	source := []byte(`
		fetch("/api/v1/users?token=secret&id=7");
		const socket = "wss://socket.example.test/events?ticket=secret";
		const user = ` + "`/api/users/${userId}/orders`" + `;
		const route = "../admin/settings";
		const image = "/assets/logo.svg";
		const bundle = "https://cdn.example.test/app.js";
		const runtime = "data:text/plain,ignored";
	`)

	var values []string
	JavaScriptEndpoints(source, "https://app.example.test/static/common.js", func(value string) {
		values = append(values, value)
	}, func(err error) {
		t.Fatalf("unexpected parse warning: %v", err)
	})
	sort.Strings(values)

	want := []string{
		"https://app.example.test/admin/settings",
		"https://app.example.test/api/users/{userId}/orders",
		"https://app.example.test/api/v1/users?id=&token=",
		"wss://socket.example.test/events?ticket=",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %v, want %v", values, want)
	}
}

func TestJavaScriptEndpointsFallsBackOnBrokenSource(t *testing.T) {
	var values []string
	JavaScriptEndpoints([]byte(`const broken = ; fetch('/api/recovery');`), "https://target.example/app.js", func(value string) {
		values = append(values, value)
	}, func(error) {})
	if len(values) != 1 || values[0] != "https://target.example/api/recovery" {
		t.Fatalf("unexpected fallback values: %v", values)
	}
}

func TestRobotsEndpointsExtractsHistoricalPaths(t *testing.T) {
	data := []byte(`
		User-agent: *
		Disallow: /old-admin/
		Allow: /old-admin/health?debug=1
		Noindex: private/export
		Sitemap: https://target.example/legacy-sitemap.xml
		Clean-param: ref&utm_source /catalog
		Crawl-delay: 10
		Disallow:
	`)
	var values []string
	RobotsEndpoints(data, "https://target.example/robots.txt", func(value string) {
		values = append(values, value)
	})
	want := []string{
		"https://target.example/old-admin/",
		"https://target.example/old-admin/health?debug=1",
		"https://target.example/private/export",
		"https://target.example/legacy-sitemap.xml",
		"https://target.example/catalog",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %v, want %v", values, want)
	}
}
