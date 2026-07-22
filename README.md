# narrowmap

`narrowmap` is a local-first Go CLI for focused narrow-recon automation.

The current `v0.4.0` scope includes:

- HTTP(S) links and their responses
- Downloaded HTML files
- Downloaded JavaScript files
- JSON responses or files
- Recursively scanned folders
- Smart target-specific parameter wordlist generation
- Historical endpoint discovery from archived `robots.txt` versions
- Historical endpoint discovery from archived JavaScript response contexts

It does not execute JavaScript. Archive modes inspect response bodies in memory
and do not save JavaScript files.

## Install

Requirements:

- Go 1.25 or newer
- [`waybackurls`](https://github.com/tomnomnom/waybackurls) for `robofinder` and
  `oJs` only

Install the archive dependency when those modes are needed:

```bash
go install github.com/tomnomnom/waybackurls@latest
```

Build one binary:

```bash
git clone <repository-url>
cd narrowmap
go build -o narrowmap ./cmd/narrowmap
```

Install it into your Go binary directory:

```bash
go install ./cmd/narrowmap
```

Make sure `$(go env GOPATH)/bin` is in `PATH`.

## Quick Start

Analyze URLs:

```bash
narrowmap --input-links links.txt
narrowmap --input-url https://target.example
narrowmap --input-url target.example
```

Analyze a folder containing downloaded HTML, JavaScript, or JSON:

```bash
narrowmap --input-folder pages_folder
```

Analyze one file:

```bash
narrowmap --input-file app.js
```

Read from stdin:

```bash
cat links.txt | narrowmap --input-links
echo target.example | narrowmap --input-url
cat app.js | narrowmap --input-file
```

Stream unique parameters as they are discovered:

```bash
narrowmap --input-links links.txt --silent
```

Write the final sorted list:

```bash
narrowmap --input-folder pages_folder -o params.txt
```

Generate a target-specific fuzzing wordlist from observed parameters:

```bash
narrowmap --paramgen params.txt -o generated-params.txt
cat params.txt | narrowmap --paramgen --silent
```

Find endpoints in old `robots.txt` versions:

```bash
narrowmap --robofinder target.example -o old-robots-endpoints.txt
```

Find endpoints inside old JavaScript response contexts:

```bash
narrowmap --ojs target.example -o old-js-endpoints.txt
cat subdomains.txt | narrowmap --oJs --archive-delay 3s -o old-js-endpoints.txt
```

`-v-param` remains accepted but is optional because visible parameter discovery
is currently the default mode.

## Paramgen

`paramgen` expands parameters observed on a target or related subdomains into a
bounded, high-signal wordlist. It is a local-only mode and never makes HTTP
requests.

The generator:

- Keeps every valid input parameter
- Deduplicates and sorts the final output
- Infers `snake_case`, `camelCase`, `kebab-case`, `dot.separated`, and bracket
  styles
- Uses curated semantic families for identifiers, accounts, authentication,
  navigation, pagination, search, files, dates, commerce, and localization
- Learns repeated target-specific namespace prefixes
- Replaces terminal words while preserving stable target namespaces, for
  example `yahoo_home_ui` to `yahoo_home_redirect`
- Reuses observed terminal words only inside the same target namespace
- Rejects URLs, assignments, malformed names, hashes, minified tokens, and
  common framework/runtime identifiers
- Avoids duplicate affixes such as `user_id_id`
- Caps generation per seed to prevent Cartesian-product growth
- Emits one-position `FUZZ` templates for compound parameters while preserving
  their style, including `FUZZ_param`, `test_FUZZ`, `FUZZ.param`,
  `test.FUZZ`, `FUZZ-param`, and `test-FUZZ`

Typical workflow:

```bash
# Collect observed parameters from authorized target material.
narrowmap --input-folder downloaded-target -o observed-params.txt

# Expand the observed vocabulary for endpoint fuzzing.
narrowmap \
  --paramgen observed-params.txt \
  --paramgen-limit 64 \
  -o target-param-wordlist.txt
```

Use the same observed list across related, authorized subdomains to build one
mass-hunt wordlist:

```bash
sort -u subdomain-results/*.params > target-family-params.txt
narrowmap --paramgen target-family-params.txt -o target-family-wordlist.txt
```

Add organization- or application-specific affixes without changing the built-in
curated set:

```bash
narrowmap \
  --paramgen observed-params.txt \
  --paramgen-prefixes prefixes.txt \
  --paramgen-suffixes suffixes.txt \
  -o target-param-wordlist.txt
```

Prefix and suffix files contain one word or short compound per line. Custom
affixes are normalized to the naming styles found in the seed list. The default
limit is `64` generated values per valid seed; accepted values are `1` through
`1000`.

## Robofinder and oJs

Both archive modes call tomnomnom's `waybackurls`. Narrowmap first inventories
archived original URLs one target at a time. It then calls
`waybackurls -get-versions` for each relevant file, which returns distinct
archived versions deduplicated by content digest.

`robofinder`:

1. Includes both HTTP and HTTPS root `robots.txt` histories for each input target.
2. Adds any other archived `robots.txt` URLs found by `waybackurls`, including
   related subdomains unless `--archive-no-subs` is set.
3. Requests raw Wayback replay contexts serially.
4. Extracts `Allow`, `Disallow`, `Noindex`, `Sitemap`, and `Clean-param` paths.
5. Resolves paths against the original host and prints full, unique endpoints.

`oJs`:

1. Filters the `waybackurls` inventory to `.js`, `.mjs`, `.cjs`, and `.jsx`
   original URLs.
2. Requests each distinct archived version serially and keeps only its response
   body in memory.
3. Statically parses string literals and route templates without executing code.
4. Resolves relative paths against the original JavaScript URL, not
   `web.archive.org`.
5. Removes fragments, blanks query values while retaining query names, and
   suppresses common image, font, media, bundle, source-map, and stylesheet
   paths.

Archive processing is deliberately serial. Its defaults are:

- Delay before every `waybackurls` call or replay request: `2s`
- Minimum accepted archive delay: `500ms`
- Per-action timeout: `90s`
- Transient retries for network errors, `429`, and common `5xx` responses: `3`
- Distinct archived versions per file: all (`--archive-max-versions 0`)

For an even slower run:

```bash
narrowmap --ojs target.example --archive-delay 5s --archive-timeout 2m
```

Large histories can take a long time because the default keeps every distinct
archived response. To bound a run while retaining coverage across time, set a
maximum. Narrowmap samples evenly from the first through latest versions; a
limit of `1` selects the latest version:

```bash
narrowmap --ojs target.example --archive-max-versions 5
```

Archive replay redirects are not allowed to leave `web.archive.org`, so these
modes do not fall back to requesting archived paths from the live target.
Custom `-H` headers are rejected in archive modes to prevent target cookies or
authorization tokens from being sent to archive.org. JavaScript response bodies
are never written to disk; only the final endpoint list is written when `-o` is
used.

## HTTP Control

The normal live URL mode uses conservative defaults:

- Concurrency: `3`
- Delay between request starts: `250ms`
- Request timeout: `20s`
- Maximum response size: `10MB`

Change them explicitly:

```bash
narrowmap \
  --input-links links.txt \
  --threads 2 \
  --delay 750ms \
  --timeout 30s
```

Add repeatable headers:

```bash
narrowmap \
  --input-links links.txt \
  -H 'Authorization: Bearer REDACTED' \
  -H 'Cookie: session=REDACTED'
```

Command-line headers may remain in shell history. Use them only on authorized
targets and handle credentials accordingly.

## What It Extracts

### HTML

- `id` values from `<input>` elements only
- `name` values from every HTML element
- Query parameter names from `href`, `src`, `action`, `formaction`, `data`, and
  `poster` URLs
- Inline JavaScript identifiers and object keys
- Recursive keys from inline JSON script blocks
- JavaScript event-handler content

When URL input returns HTML, narrowmap also fetches referenced **same-origin**
JavaScript assets using the same headers, concurrency, delay, timeout, and size
limit by default. Control this behavior explicitly:

```bash
narrowmap --input-links links.txt --no-same-origin-js
narrowmap --input-url target.example --same-origin-js=false
```

Use `--include-same-origin-js` to make the default explicit. It does not
automatically fetch cross-origin scripts. Add authorized cross-origin script
URLs to `links.txt` explicitly.

### JavaScript

- Parameter-like variable names
- Function parameter names
- JSON-like object literal keys, including low-signal keys such as `obj_val`
- Destructuring keys and bindings
- Query parameter names found in URL strings

Member access names such as `image.currentSrc`, `response.status`, and
`client.callMethod` are not extracted.

The default filter rejects:

- Single-character and minified identifiers
- Standard runtime globals
- Framework lifecycle names such as `wrapRootElement`
- React/Gatsby/Webpack layout and component plumbing
- Function names and function-valued object callbacks
- Generic implementation names such as `pageConfig`, `response`, and
  `requestOptions`
- JavaScript reserved words, global constructors, browser globals, and
  built-in method names
- Class, constructor, exported member, and enum names
- Member names such as `addEventListener`, `callMethod`, `currentSrc`,
  `forEach`, `status`, and `Object.keys`
- DOM/runtime properties such as `currentSrc`, `readyState`, and `parentNode`
- Framework execution queues such as `callQueue`, `callbackQueue`, and
  `taskQueue`
- React/Preact and Vue-style hooks matching `use[A-Z]`, including custom and
  future hooks
- Lifecycle, compiler, and rendering APIs from Next/Remix, Vue/Nuxt, Angular,
  Svelte/SvelteKit, Solid, Qwik, Astro, Lit, Alpine, Stencil, HTMX, and Gatsby
- Library metadata such as `$$typeof`, `AxiosHeaders`, `ERR_BAD_REQUEST`,
  `HttpStatusCode`, `ERR_*` constants, and HTTP status enum labels

Names with parameter signals such as `user_id`, `accountId`, `redirect_to`,
`api_token`, `email`, `cursor`, `amount`, `role`, `file`, `webhook`, and
`callback` are retained. The hook rule does not reject names such as `userId`
or `use_id`.

Use the broad compatibility mode when investigating a missed candidate:

```bash
narrowmap --input-file app.js --all-params
```

`--all-params` restores low-signal variable names. Valid object keys are already
kept by default. The flag never enables member access names, class names, library
metadata, or IDs from non-input HTML elements. If strict parsing fails, a
conservative static fallback keeps declarations, object keys, and URL query
names. JavaScript is never executed.

### JSON

- Every object key, recursively
- Query parameter names found inside string URLs

### HTTP

- Query parameter names from requested and final redirect URLs
- Query parameter names in `Location`, `Content-Location`, `Link`, and `Refresh`
  response headers
- Cookie names from supplied `Cookie` headers and response `Set-Cookie` headers
- Parameter candidates extracted from HTML, JavaScript, or JSON response bodies

Raw POST bodies, HAR files, and Burp request/response exports are not part of
`v0.4.0`; they are planned as separate input modes.

## Output Contract

Normal mode:

- Findings are deduplicated and sorted.
- Parameter names or archive endpoints go to stdout, depending on the mode.
- Stages and warnings go to stderr.

This keeps pipelines stable:

```bash
narrowmap --input-file page.html | tee params.txt
```

Silent mode:

- Each unique finding is printed to stdout immediately on discovery.
- Progress stages are disabled.
- Network or parsing warnings still use stderr when action is needed.
- Extraction modes use discovery order.
- `paramgen` remains sorted and deterministic because it generates locally
  before writing stdout.

Use `-o params.txt` when silent streaming and a final sorted artifact are both
required.

## Supported Local Files

- `.html`, `.htm`, `.xhtml`
- `.js`, `.mjs`, `.cjs`, `.jsx`
- `.json`

Folder mode skips symlinks and unsupported file extensions.

## Current Roadmap

The parameter workflow is intentionally staged:

1. Visible/context parameter discovery: implemented
2. Raw HTTP, HAR, and Burp import
3. Wayback JavaScript endpoint discovery: implemented as `oJs`
4. Wayback robots endpoint discovery: implemented as `robofinder`
5. Reused parameter corpora across related assets
6. Target-specific parameter permutations: implemented as `paramgen`
7. Controlled parameter fuzzing and response comparison

Each mode keeps one clean output type: parameter modes print parameter names,
while archive modes print normalized endpoints. Future controlled-fuzzing and
reporting modes should retain provenance so observed, archived, reused, and
generated candidates never receive the same confidence.

## Development

```bash
go test ./...
go vet ./...
go build -o bin/narrowmap ./cmd/narrowmap
```

Use only against systems you own or are explicitly authorized to test.
