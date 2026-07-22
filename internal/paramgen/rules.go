package paramgen

type affixRule struct {
	prefixes []string
	suffixes []string
}

var rulesByToken = map[string]affixRule{
	"id": {
		prefixes: []string{"user", "account", "customer", "member", "profile", "organization", "tenant", "team", "project", "resource", "object", "parent", "owner"},
	},
	"ids": {
		prefixes: []string{"user", "account", "customer", "member", "organization", "tenant", "team", "project", "resource", "object"},
	},
	"uuid": {
		prefixes: []string{"user", "account", "customer", "organization", "tenant", "project", "resource", "object"},
	},
	"user": {
		prefixes: []string{"current", "target", "parent", "owner", "admin"},
		suffixes: []string{"id", "ids", "uuid", "name", "email", "role", "type", "status", "token"},
	},
	"account": {
		prefixes: []string{"current", "target", "parent", "billing", "primary"},
		suffixes: []string{"id", "ids", "uuid", "name", "email", "type", "status", "role", "token", "balance"},
	},
	"customer": {
		prefixes: []string{"current", "target", "parent", "billing"},
		suffixes: []string{"id", "ids", "uuid", "name", "email", "type", "status"},
	},
	"member": {
		prefixes: []string{"current", "target", "team", "organization"},
		suffixes: []string{"id", "ids", "uuid", "name", "email", "role", "status"},
	},
	"profile": {
		prefixes: []string{"user", "account", "public", "current"},
		suffixes: []string{"id", "uuid", "name", "type", "status", "url"},
	},
	"organization": {
		prefixes: []string{"current", "target", "parent", "owner"},
		suffixes: []string{"id", "ids", "uuid", "name", "slug", "type", "status"},
	},
	"org": {
		prefixes: []string{"current", "target", "parent", "owner"},
		suffixes: []string{"id", "ids", "uuid", "name", "slug", "type", "status"},
	},
	"tenant": {
		prefixes: []string{"current", "target", "parent", "owner"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "slug", "status"},
	},
	"team": {
		prefixes: []string{"current", "target", "parent", "owner"},
		suffixes: []string{"id", "ids", "uuid", "name", "slug", "role", "status"},
	},
	"project": {
		prefixes: []string{"current", "target", "parent", "owner", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "slug", "type", "status"},
	},
	"resource": {
		prefixes: []string{"current", "target", "parent", "owner", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "type", "status"},
	},
	"object": {
		prefixes: []string{"current", "target", "parent", "owner", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "type", "status"},
	},
	"item": {
		prefixes: []string{"current", "target", "parent", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "type", "status", "count"},
	},
	"record": {
		prefixes: []string{"current", "target", "parent", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "key", "type", "status"},
	},
	"email": {
		prefixes: []string{"user", "account", "contact", "billing", "primary", "new", "old"},
		suffixes: []string{"id", "address", "status", "verified", "confirmation"},
	},
	"phone": {
		prefixes: []string{"user", "account", "contact", "billing", "primary", "mobile"},
		suffixes: []string{"id", "number", "country", "code", "verified"},
	},
	"address": {
		prefixes: []string{"user", "account", "billing", "shipping", "primary"},
		suffixes: []string{"id", "line", "type", "country", "city", "state", "zip", "postal_code"},
	},
	"name": {
		prefixes: []string{"user", "account", "display", "first", "last", "full", "file"},
		suffixes: []string{"id", "type", "prefix", "suffix"},
	},
	"token": {
		prefixes: []string{"access", "refresh", "api", "auth", "session", "csrf", "reset", "verification"},
		suffixes: []string{"id", "type", "value", "expires", "expiry"},
	},
	"key": {
		prefixes: []string{"api", "access", "auth", "public", "private", "client", "secret"},
		suffixes: []string{"id", "name", "type", "value", "version"},
	},
	"secret": {
		prefixes: []string{"api", "auth", "client", "shared", "webhook"},
		suffixes: []string{"id", "key", "token", "value", "version"},
	},
	"code": {
		prefixes: []string{"auth", "access", "invite", "reset", "verification", "promo", "coupon", "country"},
		suffixes: []string{"id", "type", "value", "expires"},
	},
	"otp": {
		prefixes: []string{"auth", "login", "email", "phone", "verification"},
		suffixes: []string{"code", "token", "type", "expires"},
	},
	"password": {
		prefixes: []string{"current", "new", "old", "confirm", "reset"},
		suffixes: []string{"hash", "confirmation", "token"},
	},
	"url": {
		prefixes: []string{"redirect", "return", "callback", "next", "continue", "webhook", "avatar", "image"},
		suffixes: []string{"id", "type", "path"},
	},
	"uri": {
		prefixes: []string{"redirect", "return", "callback", "next", "continue", "webhook"},
		suffixes: []string{"id", "type", "path"},
	},
	"path": {
		prefixes: []string{"redirect", "return", "callback", "next", "continue", "file", "image"},
		suffixes: []string{"id", "type", "url"},
	},
	"redirect": {
		prefixes: []string{"login", "logout", "auth", "success", "failure"},
		suffixes: []string{"url", "uri", "path", "to"},
	},
	"callback": {
		prefixes: []string{"auth", "oauth", "payment", "webhook", "success", "failure"},
		suffixes: []string{"url", "uri", "path", "token"},
	},
	"webhook": {
		prefixes: []string{"payment", "event", "callback", "notification"},
		suffixes: []string{"id", "url", "uri", "token", "secret", "type"},
	},
	"page": {
		prefixes: []string{"current", "next", "previous", "start"},
		suffixes: []string{"id", "number", "size", "limit", "offset", "cursor", "count", "total"},
	},
	"cursor": {
		prefixes: []string{"next", "previous", "start", "end", "page"},
		suffixes: []string{"id", "token", "value", "limit"},
	},
	"offset": {
		prefixes: []string{"page", "start", "result"},
		suffixes: []string{"value", "limit", "count"},
	},
	"limit": {
		prefixes: []string{"page", "result", "rate", "max"},
		suffixes: []string{"value", "count", "offset"},
	},
	"search": {
		prefixes: []string{"user", "account", "global", "advanced"},
		suffixes: []string{"query", "term", "text", "filter", "sort", "fields", "limit"},
	},
	"query": {
		prefixes: []string{"search", "filter", "user", "global"},
		suffixes: []string{"id", "text", "string", "term", "type", "fields"},
	},
	"filter": {
		prefixes: []string{"search", "user", "account", "advanced"},
		suffixes: []string{"id", "name", "type", "value", "field", "fields", "operator"},
	},
	"sort": {
		prefixes: []string{"default", "primary", "secondary"},
		suffixes: []string{"by", "field", "order", "direction", "type"},
	},
	"order": {
		prefixes: []string{"current", "parent", "purchase", "sort", "display", "default"},
		suffixes: []string{"id", "ids", "uuid", "number", "type", "status", "amount", "total", "by", "field", "direction"},
	},
	"date": {
		prefixes: []string{"start", "end", "from", "to", "created", "updated", "expires"},
		suffixes: []string{"from", "to", "before", "after", "format"},
	},
	"time": {
		prefixes: []string{"start", "end", "from", "to", "created", "updated", "expires"},
		suffixes: []string{"from", "to", "before", "after", "zone", "format"},
	},
	"file": {
		prefixes: []string{"upload", "source", "target", "attachment", "document"},
		suffixes: []string{"id", "ids", "uuid", "name", "type", "size", "url", "path", "hash"},
	},
	"image": {
		prefixes: []string{"profile", "avatar", "cover", "upload", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "type", "size", "url", "path"},
	},
	"status": {
		prefixes: []string{"user", "account", "order", "payment", "invoice", "subscription", "request"},
	},
	"state": {
		prefixes: []string{"user", "account", "order", "payment", "request", "workflow"},
	},
	"type": {
		prefixes: []string{"user", "account", "object", "resource", "file", "event", "payment"},
	},
	"role": {
		prefixes: []string{"user", "account", "member", "organization", "project"},
		suffixes: []string{"id", "ids", "name", "type", "scope"},
	},
	"amount": {
		prefixes: []string{"payment", "order", "invoice", "total", "tax", "discount"},
		suffixes: []string{"value", "currency", "min", "max"},
	},
	"payment": {
		prefixes: []string{"current", "default", "source", "target"},
		suffixes: []string{"id", "ids", "uuid", "method", "type", "status", "amount", "token"},
	},
	"invoice": {
		prefixes: []string{"current", "billing", "payment"},
		suffixes: []string{"id", "ids", "uuid", "number", "type", "status", "amount", "total"},
	},
	"product": {
		prefixes: []string{"current", "parent", "source"},
		suffixes: []string{"id", "ids", "uuid", "name", "sku", "type", "status", "price"},
	},
	"locale": {
		prefixes: []string{"user", "account", "default", "fallback"},
		suffixes: []string{"id", "code", "name"},
	},
	"language": {
		prefixes: []string{"user", "account", "default", "fallback"},
		suffixes: []string{"id", "code", "name"},
	},
	"country": {
		prefixes: []string{"user", "account", "billing", "shipping", "default"},
		suffixes: []string{"id", "code", "name"},
	},
	"currency": {
		prefixes: []string{"user", "account", "payment", "billing", "default"},
		suffixes: []string{"id", "code", "name", "symbol"},
	},
}

var knownPrefixes = stringSet(
	"access", "admin", "api", "auth", "billing", "callback", "contact",
	"current", "default", "destination", "display", "external", "failure",
	"internal", "new", "next", "old", "owner", "parent", "previous",
	"primary", "private", "public", "refresh", "reset", "return", "shipping",
	"source", "success", "target", "verification", "webhook",
)

var knownSuffixes = stringSet(
	"active", "address", "amount", "at", "balance", "by", "code", "codes",
	"confirmation", "count", "cursor", "date", "direction", "enabled",
	"expires", "expiry", "field", "fields", "format", "hash", "id", "ids",
	"key", "keys", "limit", "list", "method", "name", "names", "number",
	"offset", "order", "page", "path", "price", "ref", "refs", "role",
	"roles", "scope", "size", "slug", "state", "status", "term", "text",
	"time", "to", "token", "tokens", "total", "type", "types", "uri", "url",
	"uuid", "value", "verified", "version",
)

// Interface-oriented leaves often expose a stable target namespace while the
// backend uses a different terminal word, such as yahoo_home_ui and
// yahoo_home_redirect.
var interfaceLeaves = stringSet(
	"button", "client", "component", "display", "form", "frontend", "layout",
	"screen", "ui", "view", "widget",
)

var interfaceLeafReplacements = []string{
	"redirect", "callback", "url", "uri", "path", "route", "target", "source",
	"action", "status", "state", "mode", "type", "id", "key", "token",
	"config", "settings", "data", "info", "enabled", "active", "version",
}

var genericLeafReplacements = []string{
	"redirect", "callback", "url", "path", "target", "action",
	"status", "mode", "type", "id", "config", "enabled",
}

var siblingLeafReplacements = map[string][]string{
	"id":       {"ids", "uuid", "key", "ref", "slug"},
	"ids":      {"id", "list", "count"},
	"uuid":     {"id", "key", "ref"},
	"key":      {"id", "token", "secret", "code"},
	"token":    {"key", "secret", "code", "state"},
	"secret":   {"key", "token", "code"},
	"code":     {"token", "key", "state"},
	"url":      {"redirect", "callback", "uri", "path", "route", "target"},
	"uri":      {"redirect", "callback", "url", "path", "route", "target"},
	"path":     {"redirect", "callback", "url", "uri", "route", "target"},
	"route":    {"redirect", "callback", "url", "uri", "path", "target"},
	"redirect": {"callback", "url", "uri", "path", "route", "target", "return", "next"},
	"callback": {"redirect", "url", "uri", "path", "route", "webhook"},
	"status":   {"state", "type", "mode", "enabled", "active"},
	"state":    {"status", "type", "mode", "enabled"},
	"type":     {"status", "state", "mode", "kind", "category"},
	"mode":     {"type", "state", "status"},
	"name":     {"slug", "label", "title", "key"},
	"slug":     {"name", "key", "id"},
	"page":     {"cursor", "offset", "limit", "size", "count"},
	"cursor":   {"page", "offset", "limit"},
	"offset":   {"page", "cursor", "limit"},
	"limit":    {"page", "cursor", "offset", "size"},
}

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
