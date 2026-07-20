package extract

import (
	"reflect"
	"sort"
	"testing"

	"narrowmap/internal/model"
)

func TestHTMLExtractsVisibleNamesLinksAndInlineScripts(t *testing.T) {
	data := []byte(`
		<html>
		  <form action="/search?form_query=one">
		    <input id="email-field" name="email">
		    <div id="account-panel" name="ignored-but-visible"></div>
		    <a href="/orders?order_id=42&include=items">orders</a>
		  </form>
		  <script>
		    const accountState = { user_id: 1, "redirect_to": "/next?return_to=home" };
		    function loadInvoice(invoice_id) { return accountState.user_id; }
		  </script>
		</html>
	`)
	var got []string
	if err := Document(model.Document{
		Name: "page.html",
		Kind: model.KindHTML,
		Body: data,
	}, Options{}, gotAdder(&got), func(error) {}); err != nil {
		t.Fatal(err)
	}

	sort.Strings(got)
	want := []string{
		"accountState",
		"email",
		"email-field",
		"form_query",
		"include",
		"invoice_id",
		"order_id",
		"redirect_to",
		"return_to",
		"user_id",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestJavaScriptExtractsDeclarationsObjectsAndMembers(t *testing.T) {
	data := []byte(`
		const requestOptions = {
		  headers: { "x-api-key": tokenValue },
		  body: { account_id: accountId, "display-name": displayName }
		};
		const response = await fetch("/api/items?cursor=next_cursor", requestOptions);
		response.json().then(({ item_id, owner_id }) => item_id);
		const item = response.data;
		item.profile_id;
	`)
	var got []string
	JavaScript(data, "", Options{}, gotAdder(&got), func(error) {})
	sort.Strings(got)

	for _, expected := range []string{
		"account_id", "body", "cursor", "display-name", "headers",
		"item_id", "owner_id", "profile_id", "x-api-key",
	} {
		if !contains(got, expected) {
			t.Errorf("missing %q in %v", expected, got)
		}
	}
	for _, rejected := range []string{"accountId", "item", "requestOptions", "response", "tokenValue"} {
		if contains(got, rejected) {
			t.Errorf("unexpected low-signal identifier %q in %v", rejected, got)
		}
	}
}

func TestJSONExtractsNestedKeysAndURLParameters(t *testing.T) {
	data := []byte(`{
	  "user": {"user_id": 12, "profile": {"avatar_url": "/avatar?size=large"}},
	  "items": [{"item_id": 1}]
	}`)
	var got []string
	if err := JSON(data, "https://example.test", gotAdder(&got)); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"user", "user_id", "profile", "avatar_url", "size", "items", "item_id"} {
		if !contains(got, expected) {
			t.Errorf("missing %q in %v", expected, got)
		}
	}
}

func TestJavaScriptFallbackKeepsUsefulCandidatesOnParseError(t *testing.T) {
	var warnings []error
	var got []string
	JavaScript([]byte(`const userToken = { user_id: value;`), "", Options{}, gotAdder(&got), func(err error) {
		warnings = append(warnings, err)
	})
	if len(warnings) == 0 {
		t.Fatal("expected parse warning")
	}
	for _, expected := range []string{"userToken", "user_id"} {
		if !contains(got, expected) {
			t.Errorf("missing fallback candidate %q in %v", expected, got)
		}
	}
}

func TestJavaScriptFilterRejectsFrameworkLifecycleNames(t *testing.T) {
	data := []byte(`
		const pageConfig = { componentName: "Home", user_id: 7 };
		exports.wrapRootElement = ({ element }) => element;
		const internalState = useState(false);
		exports.onCreatePage = ({ page }) => page;
		const userToken = "redacted";
		const requestPayload = { account_id: 1, redirect_to: "/home" };
	`)

	var filtered []string
	JavaScript(data, "", Options{}, gotAdder(&filtered), func(error) {})
	for _, expected := range []string{"account_id", "redirect_to", "requestPayload", "userToken", "user_id"} {
		if !contains(filtered, expected) {
			t.Errorf("missing useful candidate %q in %v", expected, filtered)
		}
	}
	for _, rejected := range []string{
		"componentName", "element", "onCreatePage", "pageConfig", "useState", "wrapRootElement",
	} {
		if contains(filtered, rejected) {
			t.Errorf("unexpected framework candidate %q in %v", rejected, filtered)
		}
	}

	var broad []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&broad), func(error) {})
	for _, expected := range []string{"componentName", "pageConfig", "wrapRootElement"} {
		if !contains(broad, expected) {
			t.Errorf("--all-params should retain %q in %v", expected, broad)
		}
	}
}

func TestHTMLAllParamsRestoresNonFormIDs(t *testing.T) {
	data := []byte(`<div id="gatsby-focus-wrapper" name="layoutNode"></div><input name="email">`)
	var filtered []string
	if err := HTML(data, "", Options{}, gotAdder(&filtered), func(error) {}); err != nil {
		t.Fatal(err)
	}
	if contains(filtered, "gatsby-focus-wrapper") || contains(filtered, "layoutNode") {
		t.Fatalf("default HTML filter retained layout identifiers: %v", filtered)
	}

	var broad []string
	if err := HTML(data, "", Options{IncludeLowSignal: true}, gotAdder(&broad), func(error) {}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"gatsby-focus-wrapper", "layoutNode", "email"} {
		if !contains(broad, expected) {
			t.Errorf("--all-params should retain %q in %v", expected, broad)
		}
	}
}

func TestNormalizeCandidateRejectsValues(t *testing.T) {
	for _, value := range []string{"", "123", "not a parameter", "https://example.test/a"} {
		if _, ok := normalizeCandidate(value); ok {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	if value, ok := normalizeCandidate("user[email]"); !ok || value != "user[email]" {
		t.Fatalf("expected bracketed parameter to be accepted, got %q, %v", value, ok)
	}
}

func gotAdder(values *[]string) func(string) {
	return func(value string) {
		if value == "" {
			return
		}
		for _, existing := range *values {
			if existing == value {
				return
			}
		}
		*values = append(*values, value)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
