package extract

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var ignoredJSNames = map[string]struct{}{
	"arguments":                     {},
	"addeventlistener":              {},
	"appendchild":                   {},
	"atob":                          {},
	"axiosheaders":                  {},
	"badrequest":                    {},
	"btoa":                          {},
	"callmethod":                    {},
	"callqueue":                     {},
	"callbackqueue":                 {},
	"canceltoken":                   {},
	"cancelanimationframe":          {},
	"childnodes":                    {},
	"classlist":                     {},
	"clearinterval":                 {},
	"cleartimeout":                  {},
	"cloneelement":                  {},
	"commitqueue":                   {},
	"console":                       {},
	"content-type":                  {},
	"constructor":                   {},
	"createelement":                 {},
	"createpages":                   {},
	"createresolvers":               {},
	"createschemacustomization":     {},
	"currentsrc":                    {},
	"currenttarget":                 {},
	"dataset":                       {},
	"document":                      {},
	"dispatchevent":                 {},
	"encodeuri":                     {},
	"encodeuricomponent":            {},
	"decodeuri":                     {},
	"decodeuricomponent":            {},
	"effectqueue":                   {},
	"element":                       {},
	"err_bad_option_value":          {},
	"err_bad_request":               {},
	"err_fr_too_many_redirects":     {},
	"err_invalid_url":               {},
	"eval":                          {},
	"eventqueue":                    {},
	"exports":                       {},
	"firstchild":                    {},
	"forwardref":                    {},
	"fragment":                      {},
	"getattribute":                  {},
	"getelementbyid":                {},
	"getelementsbyclassname":        {},
	"global":                        {},
	"globalthis":                    {},
	"hasattribute":                  {},
	"hasownproperty":                {},
	"httpstatuscode":                {},
	"httpversionnotsupported":       {},
	"isfinite":                      {},
	"isnan":                         {},
	"isvalidelement":                {},
	"jobqueue":                      {},
	"lastchild":                     {},
	"length":                        {},
	"microtaskqueue":                {},
	"misdirectedrequest":            {},
	"module":                        {},
	"mozprintablekey":               {},
	"multistatus":                   {},
	"naturalheight":                 {},
	"naturalwidth":                  {},
	"networkauthenticationrequired": {},
	"nextsibling":                   {},
	"nodename":                      {},
	"nodetype":                      {},
	"nocontent":                     {},
	"nonauthoritativeinformation":   {},
	"oncliententry":                 {},
	"oncreatepage":                  {},
	"onpostprefetchpathname":        {},
	"onprerouteupdate":              {},
	"onrouteupdate":                 {},
	"outerhtml":                     {},
	"ownerdocument":                 {},
	"pageinfo":                      {},
	"parentelement":                 {},
	"parentnode":                    {},
	"parsefloat":                    {},
	"parseint":                      {},
	"partialcontent":                {},
	"payloadtoolarge":               {},
	"paymentrequired":               {},
	"permanentredirect":             {},
	"pendingqueue":                  {},
	"previoussibling":               {},
	"preventdefault":                {},
	"profiler":                      {},
	"proptypes":                     {},
	"proxyauthenticationrequired":   {},
	"queryselector":                 {},
	"queryselectorall":              {},
	"queuemicrotask":                {},
	"readystate":                    {},
	"renderqueue":                   {},
	"replacehydratefunction":        {},
	"replacerenderer":               {},
	"requestanimationframe":         {},
	"requestheaderfieldstoolarge":   {},
	"requesttimeout":                {},
	"require":                       {},
	"removechild":                   {},
	"removeeventlistener":           {},
	"setattribute":                  {},
	"setfieldsongraphqlnodetype":    {},
	"setinterval":                   {},
	"setstate":                      {},
	"settimeout":                    {},
	"sourcenodes":                   {},
	"stopimmediatepropagation":      {},
	"stoppropagation":               {},
	"structuredclone":               {},
	"suspense":                      {},
	"tagname":                       {},
	"taskqueue":                     {},
	"textcontent":                   {},
	"this":                          {},
	"undefined":                     {},
	"updatequeue":                   {},
	"usecallback":                   {},
	"usecontext":                    {},
	"useeffect":                     {},
	"usememo":                       {},
	"usereducer":                    {},
	"useref":                        {},
	"usestate":                      {},
	"window":                        {},
	"workqueue":                     {},
	"$$typeof":                      {},
	"tostring":                      {},
	"valueof":                       {},
	"__narrowmap__":                 {},
}

var jsReservedWords = map[string]struct{}{
	"await": {}, "break": {}, "case": {}, "catch": {}, "class": {},
	"const": {}, "continue": {}, "debugger": {}, "default": {}, "delete": {},
	"do": {}, "else": {}, "enum": {}, "export": {}, "extends": {},
	"false": {}, "finally": {}, "for": {}, "function": {}, "if": {},
	"implements": {}, "import": {}, "in": {}, "instanceof": {}, "interface": {},
	"let": {}, "new": {}, "null": {}, "package": {}, "private": {},
	"protected": {}, "public": {}, "return": {}, "static": {}, "super": {},
	"switch": {}, "this": {}, "throw": {}, "true": {}, "try": {},
	"typeof": {}, "var": {}, "void": {}, "while": {}, "with": {}, "yield": {},
}

var jsBuiltInIdentifiers = map[string]struct{}{
	"AbortController": {}, "Array": {}, "ArrayBuffer": {}, "Atomics": {},
	"BigInt": {}, "BigInt64Array": {}, "BigUint64Array": {}, "Blob": {},
	"Boolean": {}, "DataView": {}, "Date": {}, "DOMParser": {}, "Error": {},
	"EvalError": {}, "Event": {}, "File": {}, "FileReader": {},
	"FinalizationRegistry": {}, "Float32Array": {}, "Float64Array": {},
	"FormData": {}, "Function": {}, "Headers": {}, "Infinity": {},
	"Int8Array": {}, "Int16Array": {}, "Int32Array": {}, "Intl": {},
	"JSON": {}, "Map": {}, "Math": {}, "MutationObserver": {}, "NaN": {},
	"Node": {}, "Number": {}, "Object": {}, "Promise": {}, "Proxy": {},
	"RangeError": {}, "ReferenceError": {}, "Reflect": {}, "RegExp": {},
	"Request": {}, "Response": {}, "Set": {}, "SharedArrayBuffer": {},
	"String": {}, "Symbol": {}, "SyntaxError": {}, "TypeError": {},
	"Uint8Array": {}, "Uint8ClampedArray": {}, "Uint16Array": {},
	"Uint32Array": {}, "URIError": {}, "URL": {}, "URLSearchParams": {},
	"WeakMap": {}, "WeakRef": {}, "WeakSet": {}, "WebAssembly": {},
	"WebSocket": {}, "Worker": {}, "XMLHttpRequest": {},
}

var httpStatusJSNames = map[string]struct{}{
	"accepted":                      {},
	"alreadyreported":               {},
	"badgateway":                    {},
	"badrequest":                    {},
	"conflict":                      {},
	"continue":                      {},
	"created":                       {},
	"earlyhints":                    {},
	"expectationfailed":             {},
	"faileddependency":              {},
	"forbidden":                     {},
	"found":                         {},
	"gatewaytimeout":                {},
	"gone":                          {},
	"httpversionnotsupported":       {},
	"imateapot":                     {},
	"imused":                        {},
	"insufficientstorage":           {},
	"internalservererror":           {},
	"lengthrequired":                {},
	"locked":                        {},
	"loopdetected":                  {},
	"methodnotallowed":              {},
	"misdirectedrequest":            {},
	"movedpermanently":              {},
	"multiplechoices":               {},
	"multistatus":                   {},
	"networkauthenticationrequired": {},
	"nocontent":                     {},
	"nonauthoritativeinformation":   {},
	"notacceptable":                 {},
	"notextended":                   {},
	"notfound":                      {},
	"notimplemented":                {},
	"notmodified":                   {},
	"ok":                            {},
	"partialcontent":                {},
	"payloadtoolarge":               {},
	"paymentrequired":               {},
	"permanentredirect":             {},
	"preconditionfailed":            {},
	"preconditionrequired":          {},
	"processing":                    {},
	"proxyauthenticationrequired":   {},
	"rangenotsatisfiable":           {},
	"requestheaderfieldstoolarge":   {},
	"requesttimeout":                {},
	"resetcontent":                  {},
	"seeother":                      {},
	"serviceunavailable":            {},
	"switchingprotocols":            {},
	"temporaryredirect":             {},
	"tooearly":                      {},
	"toomanyrequests":               {},
	"unauthorized":                  {},
	"unprocessableentity":           {},
	"unsupportedmediatype":          {},
	"unused":                        {},
	"upgraderequired":               {},
	"uritoolong":                    {},
	"useproxy":                      {},
	"variantalsonegotiates":         {},
}

// Framework lifecycle, compiler, and rendering APIs are implementation details,
// not request or response parameter names.
var frameworkJSNames = map[string]struct{}{
	// React and Preact.
	"createcontext":         {},
	"createref":             {},
	"jsx":                   {},
	"jsxdev":                {},
	"jsxruntime":            {},
	"jsxs":                  {},
	"reactelement":          {},
	"starttransition":       {},
	"strictmode":            {},
	"use":                   {},
	"usingcliententrypoint": {},

	// Next.js and Remix.
	"clientaction":         {},
	"clientloader":         {},
	"errorboundary":        {},
	"generatemetadata":     {},
	"generatestaticparams": {},
	"getserversideprops":   {},
	"getstaticpaths":       {},
	"getstaticprops":       {},
	"hydratefallback":      {},
	"revalidatepath":       {},
	"revalidatetag":        {},
	"shouldrevalidate":     {},
	"unstable_cache":       {},
	"unstable_nostore":     {},

	// Vue and Nuxt.
	"abortnavigation":      {},
	"addroutermiddleware":  {},
	"clearnuxtdata":        {},
	"defineasynccomponent": {},
	"definecomponent":      {},
	"definenuxtconfig":     {},
	"definepagemeta":       {},
	"definerouterules":     {},
	"effectscope":          {},
	"navigateto":           {},
	"onactivated":          {},
	"onbeforemount":        {},
	"onbeforeunmount":      {},
	"onbeforeupdate":       {},
	"ondeactivated":        {},
	"onerrorcaptured":      {},
	"onmounted":            {},
	"onrendertracked":      {},
	"onrendertriggered":    {},
	"onserverprefetch":     {},
	"onunmounted":          {},
	"onupdated":            {},
	"onwatchercleanup":     {},
	"prefetchcomponents":   {},
	"preloadcomponents":    {},
	"refreshnuxtdata":      {},

	// Angular.
	"aftereveryrender":      {},
	"afternextrender":       {},
	"afterrendereffect":     {},
	"ngaftercontentchecked": {},
	"ngaftercontentinit":    {},
	"ngafterviewchecked":    {},
	"ngafterviewinit":       {},
	"ngdocheck":             {},
	"ngonchanges":           {},
	"ngondestroy":           {},
	"ngoninit":              {},

	// Svelte and SvelteKit.
	"afterupdate":           {},
	"beforeupdate":          {},
	"createeventdispatcher": {},
	"getallcontexts":        {},
	"getcontext":            {},
	"hascontext":            {},
	"ondestroy":             {},
	"onmount":               {},
	"setcontext":            {},

	// Solid and SolidStart.
	"createcomputed": {},
	"createeffect":   {},
	"creatememo":     {},
	"createreaction": {},
	"createresource": {},
	"createroot":     {},
	"createselector": {},
	"createsignal":   {},
	"createstore":    {},
	"createuniqueid": {},
	"oncleanup":      {},

	// Qwik.
	"component$":    {},
	"globalaction$": {},
	"routeaction$":  {},
	"routeloader$":  {},
	"server$":       {},
	"validator$":    {},

	// Astro.
	"definecollection": {},
	"defineconfig":     {},
	"getcollection":    {},
	"getentries":       {},
	"getentry":         {},
	"getviteconfig":    {},

	// Lit.
	"adoptedcallback":          {},
	"attributechangedcallback": {},
	"connectedcallback":        {},
	"createrenderroot":         {},
	"customelement":            {},
	"disconnectedcallback":     {},
	"firstupdated":             {},
	"performupdate":            {},
	"queryassignedelements":    {},
	"queryassignednodes":       {},
	"requestupdate":            {},
	"shouldupdate":             {},
	"updatecomplete":           {},

	// Alpine.js.
	"$data":     {},
	"$dispatch": {},
	"$el":       {},
	"$id":       {},
	"$nexttick": {},
	"$refs":     {},
	"$root":     {},
	"$store":    {},
	"$watch":    {},

	// Stencil.
	"componentdidload":      {},
	"componentdidrender":    {},
	"componentdidupdate":    {},
	"componentshouldupdate": {},
	"componentwillload":     {},
	"componentwillrender":   {},

	// HTMX.
	"htmx": {},

	// Gatsby.
	"wrappageelement": {},
}

var frameworkJSInternalPrefixes = []string{
	"__alpine", "__angular", "__astro", "__gatsby", "__htmx", "__lit", "__next",
	"__ng", "__nuxt", "__preact", "__qwik", "__react", "__remix",
	"__solid", "__svelte", "__vite", "__vue", "__webpack",
}

var highSignalJSFragments = []string{
	"account", "amount", "api", "auth", "billing", "body", "callback", "code",
	"content", "cookie", "coupon", "csrf", "cursor", "description",
	"download", "email", "export", "file", "filter", "format", "header",
	"id", "import", "invoice", "key", "limit", "locale", "message", "name",
	"nonce", "offset", "order", "org", "page", "param", "password", "path",
	"payload", "payment", "permission", "phone", "plan", "price", "project",
	"quantity", "query", "redirect", "refund", "request", "return", "role", "search", "secret",
	"session", "size", "slug", "sort", "state", "status", "team", "token",
	"type", "upload", "url", "uri", "user", "value", "version", "webhook",
}

var genericJSSuffixes = []string{
	"config", "context", "element", "options", "props", "provider",
	"wrapper", "wrapperprops",
}

func normalizeCandidate(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`")
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return "", false
	}

	hasNameRune := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			if unicode.IsLetter(r) {
				hasNameRune = true
			}
		case strings.ContainsRune("_-$.:[]@", r):
			if r == '_' || r == '$' {
				hasNameRune = true
			}
		default:
			return "", false
		}
	}
	if !hasNameRune {
		return "", false
	}
	return value, true
}

func normalizeJSName(value string) (string, bool) {
	return normalizeJSNameWithMode(value, false)
}

func normalizeJSVariableNameWithMode(value string, includeLowSignal bool) (string, bool) {
	value, ok := normalizeJSNameWithMode(value, includeLowSignal)
	if !ok || startsWithUppercase(value) {
		return "", false
	}
	return value, true
}

func normalizeJSObjectKeyWithMode(value string, _ bool) (string, bool) {
	// Object keys are data fields, so keep valid low-signal keys such as obj_val.
	return normalizeJSNameWithMode(value, true)
}

func normalizeJSNameWithMode(value string, includeLowSignal bool) (string, bool) {
	value, ok := normalizeCandidate(value)
	if !ok || utf8.RuneCountInString(value) < 2 {
		return "", false
	}
	lower := strings.ToLower(value)
	if _, ignored := ignoredJSNames[lower]; ignored {
		return "", false
	}
	if _, statusName := httpStatusJSNames[lower]; statusName {
		return "", false
	}
	if strings.HasPrefix(lower, "err_") {
		return "", false
	}
	if isFrameworkJSName(value) {
		return "", false
	}
	if _, reserved := jsReservedWords[value]; reserved {
		return "", false
	}
	if _, builtIn := jsBuiltInIdentifiers[value]; builtIn {
		return "", false
	}
	if !includeLowSignal && !isUsefulJSName(value) {
		return "", false
	}
	return value, true
}

func isFrameworkJSName(value string) bool {
	if hasUpperCamelPrefix(value, "use") || hasUpperCamelPrefix(value, "using") {
		return true
	}
	if strings.HasPrefix(value, "ɵ") {
		return true
	}

	lower := strings.ToLower(value)
	if _, frameworkName := frameworkJSNames[lower]; frameworkName {
		return true
	}
	for _, prefix := range frameworkJSInternalPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func hasUpperCamelPrefix(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(value[len(prefix):])
	return unicode.IsUpper(r)
}

func startsWithUppercase(value string) bool {
	r, _ := utf8.DecodeRuneInString(value)
	return unicode.IsUpper(r)
}

func isUsefulJSName(value string) bool {
	lower := strings.ToLower(value)

	if strings.HasPrefix(lower, "wrap") ||
		strings.HasPrefix(lower, "gatsby") ||
		strings.HasPrefix(lower, "webpack") ||
		strings.HasPrefix(lower, "component") ||
		strings.HasPrefix(lower, "layout") ||
		strings.HasPrefix(lower, "provider") ||
		strings.HasPrefix(lower, "render") ||
		strings.HasPrefix(lower, "root") ||
		strings.HasPrefix(lower, "style") ||
		strings.HasPrefix(lower, "theme") {
		return false
	}
	for _, suffix := range genericJSSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	switch lower {
	case "data", "error", "fetch", "json", "request", "response", "result":
		return false
	}

	for _, fragment := range highSignalJSFragments {
		if fragment == "id" {
			if lower == "id" ||
				strings.HasSuffix(lower, "_id") ||
				strings.HasSuffix(lower, "-id") ||
				strings.HasSuffix(value, "Id") ||
				strings.HasSuffix(value, "ID") {
				return true
			}
			continue
		}
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
