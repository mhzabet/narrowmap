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
		"ignored-but-visible",
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

func TestJavaScriptExtractsDeclarationsAndObjectKeys(t *testing.T) {
	data := []byte(`
		const requestOptions = {
		  headers: { "x-api-key": tokenValue },
		  body: { account_id: accountId, "display-name": displayName }
		};
		const response = await fetch("/api/items?cursor=next_cursor", requestOptions);
		response.json().then(({ item_id, owner_id }) => item_id);
		const source_url = response.data;
		item.profile_id;
		item["accountStatus"];
	`)
	var got []string
	JavaScript(data, "", Options{}, gotAdder(&got), func(error) {})
	sort.Strings(got)

	for _, expected := range []string{
		"account_id", "body", "cursor", "display-name", "headers",
		"item_id", "owner_id", "source_url", "x-api-key",
	} {
		if !contains(got, expected) {
			t.Errorf("missing %q in %v", expected, got)
		}
	}
	for _, rejected := range []string{
		"accountId", "accountStatus", "data", "profile_id", "requestOptions", "response", "tokenValue",
	} {
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
		"element", "onCreatePage", "pageConfig", "useState", "wrapRootElement",
	} {
		if contains(filtered, rejected) {
			t.Errorf("unexpected framework candidate %q in %v", rejected, filtered)
		}
	}

	var broad []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&broad), func(error) {})
	for _, expected := range []string{"componentName", "pageConfig"} {
		if !contains(broad, expected) {
			t.Errorf("--all-params should retain %q in %v", expected, broad)
		}
	}
	if contains(broad, "wrapRootElement") {
		t.Errorf("--all-params retained member property %q in %v", "wrapRootElement", broad)
	}
}

func TestJavaScriptRejectsTopFrameworkRuntimeNames(t *testing.T) {
	data := []byte(`
		const useCSSOMInjection = 1;
		const useDebugValue = 1;
		const useDecimalCurrencyValues = 1;
		const useDeferredValue = 1;
		const useId = 1;
		const useImperativeHandle = 1;
		const useInsertionEffect = 1;
		const useLayoutEffect = 1;
		const useMutableSource = 1;
		const useSyncExternalStore = 1;
		const useTransition = 1;
		const usingClientEntryPoint = 1;
		const userId = 7;
		const use_id = 8;
		const user_id = 9;
		const frameworkRuntime = {
			getStaticProps: "",
			generateMetadata: "",
			shouldRevalidate: "",
			onMounted: "",
			defineNuxtConfig: "",
			ngOnInit: "",
			ngAfterViewInit: "",
			onMount: "",
			beforeUpdate: "",
			createSignal: "",
			createEffect: "",
			component$: "",
			routeLoader$: "",
			defineCollection: "",
			getCollection: "",
			createRenderRoot: "",
			customElement: "",
			requestUpdate: "",
			componentWillLoad: "",
			connectedCallback: "",
			htmx: "",
			$nextTick: "",
			$dispatch: "",
			__reactServerRuntime: "",
			__svelteKitRuntime: "",
			ɵɵdefineComponent: "",
			obj_val: ""
		};
	`)

	var got []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&got), func(error) {})

	for _, expected := range []string{"obj_val", "use_id", "userId", "user_id"} {
		if !contains(got, expected) {
			t.Errorf("useful parameter %q was dropped from %v", expected, got)
		}
	}
	for _, rejected := range []string{
		"$dispatch", "$nextTick", "__reactServerRuntime", "__svelteKitRuntime",
		"beforeUpdate", "component$", "componentWillLoad", "connectedCallback",
		"createEffect", "createRenderRoot", "createSignal", "customElement",
		"defineCollection", "defineNuxtConfig",
		"generateMetadata", "getCollection", "getStaticProps",
		"htmx", "ngAfterViewInit", "ngOnInit", "onMount", "onMounted",
		"requestUpdate", "routeLoader$", "shouldRevalidate",
		"useCSSOMInjection", "useDebugValue", "useDecimalCurrencyValues",
		"useDeferredValue", "useId", "useImperativeHandle",
		"useInsertionEffect", "useLayoutEffect", "useMutableSource",
		"useSyncExternalStore", "useTransition", "usingClientEntryPoint",
		"ɵɵdefineComponent",
	} {
		if contains(got, rejected) {
			t.Errorf("framework runtime name %q was retained in %v", rejected, got)
		}
	}
}

func TestJavaScriptFilterRejectsBuiltInsAndCalledMethods(t *testing.T) {
	data := []byte(`
		const request_id = 7;
		const payload = { user_id: request_id, filter: "active" };
		window.addEventListener("load", callback);
		client.callMethod(payload);
		Object.keys(payload).forEach(handleItem);
		client["removeEventListener"]("load", callback);
		const result = new Promise(resolve);
	`)

	var got []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&got), func(error) {})

	for _, expected := range []string{"request_id", "user_id", "filter"} {
		if !contains(got, expected) {
			t.Errorf("missing useful candidate %q in %v", expected, got)
		}
	}
	for _, rejected := range []string{
		"addEventListener", "callMethod", "forEach", "keys",
		"removeEventListener", "Object", "Promise",
	} {
		if contains(got, rejected) {
			t.Errorf("unexpected built-in or called method %q in %v", rejected, got)
		}
	}
}

func TestJavaScriptFilterRejectsDOMRuntimeAndQueueNames(t *testing.T) {
	data := []byte(`
		const callQueue = [];
		const callbackQueue = [];
		const taskQueue = [];
		const source_url = image.currentSrc;
		const queue_id = 7;
		const payload = { request_queue: "priority", user_id: 1 };
		video.readyState;
		node.parentNode;
	`)

	var got []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&got), func(error) {})

	for _, expected := range []string{"queue_id", "request_queue", "source_url", "user_id"} {
		if !contains(got, expected) {
			t.Errorf("missing useful candidate %q in %v", expected, got)
		}
	}
	for _, rejected := range []string{
		"callQueue", "callbackQueue", "taskQueue", "currentSrc", "readyState", "parentNode",
	} {
		if contains(got, rejected) {
			t.Errorf("unexpected DOM runtime or queue name %q in %v", rejected, got)
		}
	}
}

func TestJavaScriptFallbackRejectsDOMRuntimeAndQueueNames(t *testing.T) {
	data := []byte(`const callQueue = { currentSrc: image.currentSrc, user_id: 1;`)
	var got []string
	JavaScript(data, "", Options{IncludeLowSignal: true}, gotAdder(&got), func(error) {})

	if !contains(got, "user_id") {
		t.Fatalf("fallback dropped useful parameter: %v", got)
	}
	for _, rejected := range []string{"callQueue", "currentSrc"} {
		if contains(got, rejected) {
			t.Errorf("fallback retained runtime name %q in %v", rejected, got)
		}
	}
}

func TestJavaScriptRejectsLibraryExportsAndEnumMetadata(t *testing.T) {
	data := []byte(`
		class AxiosHeaders {}
		class userModel {}
		function requestHandler() {}
		const HttpStatusCode = {
			Continue: 100,
			Created: 201,
			BadRequest: 400,
			HttpVersionNotSupported: 505,
			InternalServerError: 500,
			MisdirectedRequest: 421,
			MultiStatus: 207,
			NetworkAuthenticationRequired: 511,
			NoContent: 204,
			NonAuthoritativeInformation: 203,
			PartialContent: 206,
			PayloadTooLarge: 413,
			PaymentRequired: 402,
			PermanentRedirect: 308,
			ProxyAuthenticationRequired: 407,
			RequestHeaderFieldsTooLarge: 431,
			RequestTimeout: 408,
			ResetContent: 205,
			Unauthorized: 401,
			UriTooLong: 414,
			obj_val: ""
		};
		const metadata = {
			"$$typeof": Symbol.for("react.element"),
			"Content-Type": "application/json",
			ERR_BAD_OPTION_VALUE: "ERR_BAD_OPTION_VALUE",
			ERR_BAD_REQUEST: "ERR_BAD_REQUEST",
			ERR_FR_TOO_MANY_REDIRECTS: "ERR_FR_TOO_MANY_REDIRECTS",
			ERR_INVALID_URL: "ERR_INVALID_URL",
			ERR_UNKNOWN_LIBRARY_CODE: "ERR_UNKNOWN_LIBRARY_CODE",
			MozPrintableKey: "MozPrintableKey"
		};
		axios.CancelToken;
		React.Profiler;
		React.PropTypes;
	`)

	var got []string
	JavaScript(data, "", Options{}, gotAdder(&got), func(error) {})

	if !contains(got, "obj_val") {
		t.Fatalf("JSON-like object key was dropped: %v", got)
	}
	for _, rejected := range []string{
		"$$typeof", "AxiosHeaders", "BadRequest", "CancelToken", "Content-Type",
		"Continue", "Created", "ERR_BAD_OPTION_VALUE", "ERR_BAD_REQUEST",
		"ERR_FR_TOO_MANY_REDIRECTS", "ERR_INVALID_URL",
		"ERR_UNKNOWN_LIBRARY_CODE", "HttpStatusCode", "HttpVersionNotSupported",
		"InternalServerError", "MisdirectedRequest", "MozPrintableKey", "MultiStatus",
		"NetworkAuthenticationRequired", "NoContent",
		"NonAuthoritativeInformation", "PartialContent", "PayloadTooLarge",
		"PaymentRequired", "PermanentRedirect", "Profiler", "PropTypes",
		"ProxyAuthenticationRequired", "RequestHeaderFieldsTooLarge",
		"RequestTimeout", "ResetContent", "Unauthorized", "UriTooLong",
		"requestHandler", "userModel",
	} {
		if contains(got, rejected) {
			t.Errorf("library or enum metadata %q was retained in %v", rejected, got)
		}
	}
}

func TestHTMLOnlyExtractsInputIDsAndAllNameAttributes(t *testing.T) {
	data := []byte(`<div id="gatsby-focus-wrapper" name="layoutNode"></div><input name="email">`)
	var filtered []string
	if err := HTML(data, "", Options{}, gotAdder(&filtered), func(error) {}); err != nil {
		t.Fatal(err)
	}
	if contains(filtered, "gatsby-focus-wrapper") {
		t.Fatalf("non-input id was retained: %v", filtered)
	}
	for _, expected := range []string{"layoutNode", "email"} {
		if !contains(filtered, expected) {
			t.Errorf("missing name attribute %q in %v", expected, filtered)
		}
	}

	var broad []string
	if err := HTML(data, "", Options{IncludeLowSignal: true}, gotAdder(&broad), func(error) {}); err != nil {
		t.Fatal(err)
	}
	if contains(broad, "gatsby-focus-wrapper") {
		t.Fatalf("--all-params restored a non-input id: %v", broad)
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
