# narrowmap

`narrowmap` is a local-first Go CLI for focused narrow-recon automation.

The current `v0.2.1` scope is **filtered visible parameter discovery** from:

- HTTP(S) links and their responses
- Downloaded HTML files
- Downloaded JavaScript files
- JSON responses or files
- Recursively scanned folders

It does not execute JavaScript.

## Install

Requirements:

- Go 1.25 or newer

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

`-v-param` remains accepted but is optional because visible parameter discovery
is currently the default mode.

## HTTP Control

The URL mode uses conservative defaults:

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

- Values of `id` and `name` attributes on form-related elements
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
- Object literal keys
- Destructuring keys and bindings
- Parameter-like member property names
- Query parameter names found in URL strings

The default filter rejects:

- Single-character and minified identifiers
- Standard runtime globals
- Framework lifecycle names such as `wrapRootElement`
- React/Gatsby/Webpack layout and component plumbing
- Function names and function-valued object callbacks
- Generic implementation names such as `pageConfig`, `response`, and
  `requestOptions`

Names with parameter signals such as `user_id`, `accountId`, `redirect_to`,
`api_token`, `email`, `cursor`, `amount`, `role`, `file`, `webhook`, and
`callback` are retained.

Use the broad compatibility mode when investigating a missed candidate:

```bash
narrowmap --input-file app.js --all-params
```

`--all-params` restores low-signal JavaScript names and `id`/`name` attributes
from non-form HTML elements. If strict parsing fails, a conservative static
fallback keeps declarations, object keys, and URL query names. JavaScript is
never executed.

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
`v0.2.1`; they are planned as separate input modes.

## Output Contract

Normal mode:

- Parameters are deduplicated and sorted.
- Parameters go to stdout.
- Stages and warnings go to stderr.

This keeps pipelines stable:

```bash
narrowmap --input-file page.html | tee params.txt
```

Silent mode:

- Each unique parameter is printed to stdout immediately on discovery.
- Progress stages are disabled.
- Network or parsing warnings still use stderr when action is needed.
- Streaming order is discovery order, not sorted order.

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
3. Wayback HTML and JavaScript parameter discovery
4. Reused parameter corpora across related assets
5. Target-specific parameter permutations
6. Controlled parameter fuzzing and response comparison

The later stages should retain provenance so observed, archived, reused, and
generated parameters never receive the same confidence.

## Development

```bash
go test ./...
go vet ./...
go build -o bin/narrowmap ./cmd/narrowmap
```

Use only against systems you own or are explicitly authorized to test.
