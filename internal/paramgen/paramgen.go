package paramgen

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const defaultPerSeedLimit = 64

type Options struct {
	Prefixes     []string
	Suffixes     []string
	PerSeedLimit int
}

type Result struct {
	Values     []string
	Accepted   int
	Rejected   int
	Duplicates int
}

type nameStyle uint8

const (
	stylePlain nameStyle = iota
	styleSnake
	styleCamel
	styleKebab
	styleDot
	styleBracket
)

type parsedName struct {
	original string
	words    []string
	style    nameStyle
	expand   bool
}

type edgeParts struct {
	prefix []string
	core   []string
	suffix []string
}

type orderedSet struct {
	seen   map[string]struct{}
	values []string
	limit  int
}

func Generate(inputs []string, options Options) Result {
	limit := options.PerSeedLimit
	if limit <= 0 {
		limit = defaultPerSeedLimit
	}

	seeds, rejected, duplicates := parseInputs(inputs)
	result := Result{
		Accepted:   len(seeds),
		Rejected:   rejected,
		Duplicates: duplicates,
	}
	if len(seeds) == 0 {
		return result
	}

	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].original < seeds[j].original
	})

	styles := corpusStyles(seeds)
	learnedPrefixes, learnedSuffixes := learnAffixes(seeds)
	namespaceLeaves := learnNamespaceLeaves(seeds)
	customPrefixes := parseAffixes(options.Prefixes)
	customSuffixes := parseAffixes(options.Suffixes)
	all := make(map[string]struct{}, len(seeds)*limit)

	for _, seed := range seeds {
		local := &orderedSet{
			seen:  make(map[string]struct{}, limit),
			limit: limit,
		}
		local.add(seed.original)
		addFuzzTemplates(local, seed)
		for _, style := range stylesForSeed(seed, styles) {
			local.add(render(seed.words, style))
		}
		if !seed.expand {
			mergeGenerated(all, local)
			continue
		}

		parts := splitEdges(seed.words)
		addPrefixMutations(local, seed, styles, parts, customPrefixes)
		addSuffixMutations(local, seed, styles, parts, customSuffixes)

		leafReplacements, leafExclusive := leafReplacementCandidates(seed.words)
		addLeafMutations(local, seed, styles, leafReplacements)
		addLeafMutations(local, seed, styles, namespaceLeaves[seed.words[0]])
		if leafExclusive {
			mergeGenerated(all, local)
			continue
		}

		prefixes, suffixes := semanticAffixes(seed.words, parts.core)
		addPrefixMutations(local, seed, styles, parts, prefixes)
		addSuffixMutations(local, seed, styles, parts, suffixes)
		addCompoundMutations(local, seed, styles, parts, prefixes, suffixes)
		addPrefixMutations(local, seed, styles, parts, learnedPrefixes)
		addSuffixMutations(local, seed, styles, parts, learnedSuffixes)

		mergeGenerated(all, local)
	}

	result.Values = make([]string, 0, len(all))
	for value := range all {
		result.Values = append(result.Values, value)
	}
	sort.Strings(result.Values)
	return result
}

func mergeGenerated(all map[string]struct{}, local *orderedSet) {
	for _, value := range local.values {
		all[value] = struct{}{}
	}
}

func parseInputs(inputs []string) ([]parsedName, int, int) {
	seen := make(map[string]struct{}, len(inputs))
	seeds := make([]parsedName, 0, len(inputs))
	rejected := 0
	duplicates := 0

	for _, input := range inputs {
		value := strings.TrimSpace(input)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		if _, exists := seen[value]; exists {
			duplicates++
			continue
		}
		seen[value] = struct{}{}

		parsed, ok := parseName(value)
		if !ok {
			rejected++
			continue
		}
		seeds = append(seeds, parsed)
	}
	return seeds, rejected, duplicates
}

func parseName(value string) (parsedName, bool) {
	if !validRawName(value) {
		return parsedName{}, false
	}

	style := stylePlain
	var words []string
	switch {
	case strings.Contains(value, "["):
		style = styleBracket
		words = splitBracket(value)
	case strings.Contains(value, "_"):
		style = styleSnake
		words = strings.Split(value, "_")
	case strings.Contains(value, "-"):
		style = styleKebab
		words = strings.Split(value, "-")
	case strings.Contains(value, "."):
		style = styleDot
		words = strings.Split(value, ".")
	default:
		words = splitCamel(value)
		if len(words) > 1 {
			style = styleCamel
		}
	}

	for index := range words {
		words[index] = strings.ToLower(words[index])
	}
	if !validWords(words) || isHardNoise(value, words) {
		return parsedName{}, false
	}
	return parsedName{
		original: value,
		words:    words,
		style:    style,
		expand:   shouldExpand(value, words),
	}, true
}

func validRawName(value string) bool {
	if len(value) == 0 || len(value) > 64 || unicode.IsDigit(rune(value[0])) {
		return false
	}
	if strings.ContainsAny(value, " \t\r\n=/?&#%:;,@\\\"'`{}()<>") {
		return false
	}
	if strings.ContainsAny(value, "[]") {
		_, ok := bracketSegments(value)
		return ok
	}

	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-', r == '.':
			if index == 0 || index == len(value)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func splitBracket(value string) []string {
	segments, _ := bracketSegments(value)
	var words []string
	for _, segment := range segments {
		words = append(words, splitSegment(segment)...)
	}
	return words
}

func bracketSegments(value string) ([]string, bool) {
	firstOpen := strings.IndexByte(value, '[')
	if firstOpen <= 0 || value[len(value)-1] != ']' {
		return nil, false
	}

	base := value[:firstOpen]
	if !validSegment(base) {
		return nil, false
	}
	segments := []string{base}
	rest := value[firstOpen:]
	for len(rest) > 0 {
		if rest[0] != '[' {
			return nil, false
		}
		closeIndex := strings.IndexByte(rest, ']')
		if closeIndex <= 1 {
			return nil, false
		}
		segment := rest[1:closeIndex]
		if !validSegment(segment) {
			return nil, false
		}
		segments = append(segments, segment)
		rest = rest[closeIndex+1:]
	}
	return segments, true
}

func validSegment(value string) bool {
	if value == "" || !isASCIILetter(value[0]) {
		return false
	}
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-':
			if index == 0 || index == len(value)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func splitSegment(value string) []string {
	switch {
	case strings.Contains(value, "_"):
		return strings.Split(value, "_")
	case strings.Contains(value, "-"):
		return strings.Split(value, "-")
	case strings.Contains(value, "."):
		return strings.Split(value, ".")
	default:
		return splitCamel(value)
	}
}

func splitCamel(value string) []string {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}

	start := 0
	var words []string
	for index := 1; index < len(runes); index++ {
		current := runes[index]
		previous := runes[index-1]
		nextLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
		if unicode.IsUpper(current) && (unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextLower) {
			words = append(words, string(runes[start:index]))
			start = index
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

func validWords(words []string) bool {
	if len(words) == 0 || len(words) > 6 {
		return false
	}
	for _, word := range words {
		if word == "" || len(word) > 32 {
			return false
		}
		for _, r := range word {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return false
			}
		}
		if len(word) == 1 {
			if _, ok := allowedShortWords[word]; !ok {
				return false
			}
		}
	}
	return true
}

var allowedShortWords = stringSet("g", "h", "q", "v", "x")

var hardNoiseNames = stringSet(
	"arguments", "array", "axiosheaders", "badrequest", "class", "component",
	"console", "constructor", "document", "element", "exports", "foreach",
	"function", "global", "globalthis", "httpstatuscode", "length", "module",
	"null", "prototype", "reactelement", "requestoptions", "this", "true",
	"undefined", "window",
)

var hardNoisePrefixes = []string{
	"__angular", "__astro", "__gatsby", "__next", "__nuxt", "__preact",
	"__qwik", "__react", "__remix", "__solid", "__svelte", "__vite",
	"__vue", "__webpack",
}

var lowSignalWords = stringSet(
	"config", "context", "data", "error", "options", "props", "request",
	"response", "result",
)

var preserveOnlyNames = stringSet(
	"cf-turnstile-response",
	"g-recaptcha-response",
	"h-captcha-response",
)

func isHardNoise(value string, words []string) bool {
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "__") || strings.HasPrefix(lower, "$$") || strings.HasPrefix(lower, "err_") {
		return true
	}
	if startsUpperCamel(value) || hasUpperCamelPrefix(value, "use") || hasUpperCamelPrefix(value, "using") {
		return true
	}
	if _, noisy := hardNoiseNames[lower]; noisy {
		return true
	}
	for _, prefix := range hardNoisePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	for _, word := range words {
		if repeatedRun(word) >= 6 {
			return true
		}
		if len(word) >= 20 && containsDigit(word) {
			return true
		}
	}
	return false
}

func shouldExpand(value string, words []string) bool {
	if _, preserveOnly := preserveOnlyNames[strings.ToLower(value)]; preserveOnly {
		return false
	}
	for _, word := range words {
		if _, semantic := rulesByToken[word]; semantic {
			return true
		}
		if _, lowSignal := lowSignalWords[word]; !lowSignal {
			return true
		}
	}
	return false
}

func startsUpperCamel(value string) bool {
	if strings.ContainsAny(value, "_.-[") || value == strings.ToUpper(value) {
		return false
	}
	runes := []rune(value)
	return len(runes) > 1 && unicode.IsUpper(runes[0]) && unicode.IsLower(runes[1])
}

func hasUpperCamelPrefix(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(value[len(prefix):])
	return unicode.IsUpper(r)
}

func repeatedRun(value string) int {
	longest := 0
	current := 0
	var previous rune
	for _, r := range value {
		if r == previous {
			current++
		} else {
			previous = r
			current = 1
		}
		if current > longest {
			longest = current
		}
	}
	return longest
}

func containsDigit(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func corpusStyles(seeds []parsedName) []nameStyle {
	used := make(map[nameStyle]struct{})
	for _, seed := range seeds {
		if seed.style != stylePlain {
			used[seed.style] = struct{}{}
		}
	}
	if len(used) == 0 {
		return []nameStyle{styleSnake, styleCamel}
	}

	ordered := make([]nameStyle, 0, len(used)+1)
	for _, style := range []nameStyle{styleSnake, styleCamel, styleKebab, styleDot, styleBracket} {
		if _, ok := used[style]; ok {
			ordered = append(ordered, style)
		}
	}
	if _, brackets := used[styleBracket]; brackets {
		if _, snakes := used[styleSnake]; !snakes {
			ordered = append(ordered, styleSnake)
		}
	}
	return ordered
}

func stylesForSeed(seed parsedName, corpus []nameStyle) []nameStyle {
	styles := make([]nameStyle, 0, len(corpus)+1)
	seen := make(map[nameStyle]struct{}, len(corpus)+1)
	if seed.style != stylePlain {
		styles = append(styles, seed.style)
		seen[seed.style] = struct{}{}
	}
	for _, style := range corpus {
		if _, exists := seen[style]; exists {
			continue
		}
		styles = append(styles, style)
		seen[style] = struct{}{}
	}
	return styles
}

func render(words []string, style nameStyle) string {
	if !validMutationWords(words) {
		return ""
	}
	switch style {
	case styleSnake:
		return strings.Join(words, "_")
	case styleCamel:
		var builder strings.Builder
		builder.WriteString(words[0])
		for _, word := range words[1:] {
			builder.WriteString(upperFirst(word))
		}
		return builder.String()
	case styleKebab:
		return strings.Join(words, "-")
	case styleDot:
		return strings.Join(words, ".")
	case styleBracket:
		if len(words) != 2 {
			return ""
		}
		return words[0] + "[" + words[1] + "]"
	default:
		if len(words) == 1 {
			return words[0]
		}
		return ""
	}
}

func addFuzzTemplates(values *orderedSet, seed parsedName) {
	if len(seed.words) < 2 {
		return
	}
	if _, preserveOnly := preserveOnlyNames[strings.ToLower(seed.original)]; preserveOnly {
		return
	}

	if seed.style == styleBracket {
		addBracketFuzzTemplates(values, seed.original)
		return
	}
	for index := range seed.words {
		values.add(renderFuzzTemplate(seed.words, seed.style, index))
	}
}

func addBracketFuzzTemplates(values *orderedSet, original string) {
	segments, ok := bracketSegments(original)
	if !ok {
		return
	}
	for segmentIndex, segment := range segments {
		segmentName, ok := parseName(segment)
		if !ok {
			continue
		}
		if len(segmentName.words) == 1 {
			mutated := append([]string(nil), segments...)
			mutated[segmentIndex] = "FUZZ"
			values.add(renderBracketSegments(mutated))
			continue
		}
		for wordIndex := range segmentName.words {
			mutated := append([]string(nil), segments...)
			mutated[segmentIndex] = renderFuzzTemplate(segmentName.words, segmentName.style, wordIndex)
			if mutated[segmentIndex] == "" {
				continue
			}
			values.add(renderBracketSegments(mutated))
		}
	}
}

func renderBracketSegments(segments []string) string {
	if len(segments) < 2 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(segments[0])
	for _, nested := range segments[1:] {
		builder.WriteByte('[')
		builder.WriteString(nested)
		builder.WriteByte(']')
	}
	return builder.String()
}

func renderFuzzTemplate(words []string, style nameStyle, fuzzIndex int) string {
	if len(words) < 2 || fuzzIndex < 0 || fuzzIndex >= len(words) {
		return ""
	}
	mutated := append([]string(nil), words...)
	mutated[fuzzIndex] = "FUZZ"

	switch style {
	case styleSnake:
		return strings.Join(mutated, "_")
	case styleKebab:
		return strings.Join(mutated, "-")
	case styleDot:
		return strings.Join(mutated, ".")
	case styleCamel:
		var builder strings.Builder
		builder.WriteString(mutated[0])
		for _, word := range mutated[1:] {
			if word == "FUZZ" {
				builder.WriteString(word)
			} else {
				builder.WriteString(upperFirst(word))
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func upperFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func validMutationWords(words []string) bool {
	if !validWords(words) {
		return false
	}
	for index := 1; index < len(words); index++ {
		if words[index] == words[index-1] {
			return false
		}
	}
	return true
}

func splitEdges(words []string) edgeParts {
	parts := edgeParts{core: append([]string(nil), words...)}
	if len(parts.core) > 1 {
		if _, ok := knownPrefixes[parts.core[0]]; ok {
			parts.prefix = []string{parts.core[0]}
			parts.core = parts.core[1:]
		}
	}
	if len(parts.core) > 1 {
		last := parts.core[len(parts.core)-1]
		if _, ok := knownSuffixes[last]; ok {
			parts.suffix = []string{last}
			parts.core = parts.core[:len(parts.core)-1]
		}
	}
	return parts
}

func parseAffixes(values []string) [][]string {
	seen := make(map[string]struct{}, len(values))
	var affixes [][]string
	for _, value := range values {
		words, ok := parseAffixWords(strings.TrimSpace(value))
		if !ok {
			continue
		}
		key := strings.Join(words, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		affixes = append(affixes, words)
	}
	sort.Slice(affixes, func(i, j int) bool {
		return strings.Join(affixes[i], "\x00") < strings.Join(affixes[j], "\x00")
	})
	return affixes
}

func parseAffixWords(value string) ([]string, bool) {
	if !validRawName(value) || strings.Contains(value, "[") {
		return nil, false
	}
	var words []string
	switch {
	case strings.Contains(value, "_"):
		words = strings.Split(value, "_")
	case strings.Contains(value, "-"):
		words = strings.Split(value, "-")
	case strings.Contains(value, "."):
		words = strings.Split(value, ".")
	default:
		words = splitCamel(value)
	}
	for index := range words {
		words[index] = strings.ToLower(words[index])
	}
	return words, len(words) <= 2 && validWords(words)
}

func semanticAffixes(words, core []string) ([][]string, [][]string) {
	var prefixes [][]string
	var suffixes [][]string
	prefixSeen := make(map[string]struct{})
	suffixSeen := make(map[string]struct{})

	addRules := func(token string) {
		rule, exists := rulesByToken[token]
		if !exists {
			return
		}
		for _, prefix := range rule.prefixes {
			addAffix(&prefixes, prefixSeen, prefix)
		}
		for _, suffix := range rule.suffixes {
			addAffix(&suffixes, suffixSeen, suffix)
		}
	}
	for _, word := range core {
		addRules(word)
	}
	if len(core) == 0 || len(prefixes) == 0 && len(suffixes) == 0 {
		for _, word := range words {
			addRules(word)
		}
	}

	if len(prefixes) == 0 {
		for _, prefix := range []string{"current", "new"} {
			addAffix(&prefixes, prefixSeen, prefix)
		}
	}
	if len(suffixes) == 0 {
		if len(words) != 1 || !isKnownSuffix(words[0]) {
			for _, suffix := range []string{"id", "name", "type", "status"} {
				addAffix(&suffixes, suffixSeen, suffix)
			}
		}
	}
	return prefixes, suffixes
}

func addAffix(values *[][]string, seen map[string]struct{}, value string) {
	words, ok := parseAffixWords(value)
	if !ok {
		return
	}
	key := strings.Join(words, "\x00")
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	*values = append(*values, words)
}

func isKnownSuffix(value string) bool {
	_, ok := knownSuffixes[value]
	return ok
}

func addPrefixMutations(local *orderedSet, seed parsedName, corpus []nameStyle, parts edgeParts, prefixes [][]string) {
	for _, prefix := range prefixes {
		words := joinWords(prefix, parts.core, parts.suffix)
		addRendered(local, seed, corpus, words)
	}
}

func addSuffixMutations(local *orderedSet, seed parsedName, corpus []nameStyle, parts edgeParts, suffixes [][]string) {
	for _, suffix := range suffixes {
		words := joinWords(parts.prefix, parts.core, suffix)
		addRendered(local, seed, corpus, words)
	}
}

func addLeafMutations(
	local *orderedSet,
	seed parsedName,
	corpus []nameStyle,
	replacements [][]string,
) {
	if len(seed.words) < 2 {
		return
	}
	stem := seed.words[:len(seed.words)-1]
	for _, replacement := range replacements {
		addRendered(local, seed, corpus, joinWords(stem, replacement))
	}
}

func leafReplacementCandidates(words []string) ([][]string, bool) {
	if len(words) < 2 {
		return nil, false
	}

	leaf := words[len(words)-1]
	stem := words[:len(words)-1]
	if _, interfaceLeaf := interfaceLeaves[leaf]; interfaceLeaf {
		return parseAffixes(interfaceLeafReplacements), true
	}

	if replacements, exists := siblingLeafReplacements[leaf]; exists {
		return parseAffixes(replacements), !hasSemanticWord(stem)
	}
	if len(words) >= 3 && !hasSemanticWord(stem) {
		return parseAffixes(genericLeafReplacements), true
	}
	return nil, false
}

func hasSemanticWord(words []string) bool {
	for _, word := range words {
		if _, semantic := rulesByToken[word]; semantic {
			return true
		}
	}
	return false
}

func addCompoundMutations(
	local *orderedSet,
	seed parsedName,
	corpus []nameStyle,
	parts edgeParts,
	prefixes [][]string,
	suffixes [][]string,
) {
	if len(parts.prefix) != 0 || len(parts.suffix) != 0 || len(parts.core) == 0 {
		return
	}
	for prefixIndex, prefix := range prefixes {
		if prefixIndex >= 2 {
			break
		}
		for suffixIndex, suffix := range suffixes {
			if suffixIndex >= 3 {
				break
			}
			addRendered(local, seed, corpus, joinWords(prefix, parts.core, suffix))
		}
	}
}

func addRendered(local *orderedSet, seed parsedName, corpus []nameStyle, words []string) {
	if containsRepeatedSequence(words) {
		return
	}
	for _, style := range stylesForSeed(seed, corpus) {
		local.add(render(words, style))
	}
}

func joinWords(groups ...[]string) []string {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	words := make([]string, 0, total)
	for _, group := range groups {
		words = append(words, group...)
	}
	return words
}

func containsRepeatedSequence(words []string) bool {
	if !validMutationWords(words) {
		return true
	}
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		if _, exists := seen[word]; exists {
			return true
		}
		seen[word] = struct{}{}
	}
	return false
}

func learnAffixes(seeds []parsedName) ([][]string, [][]string) {
	prefixCounts := make(map[string]int)
	for _, seed := range seeds {
		if len(seed.words) < 2 {
			continue
		}
		prefixCounts[seed.words[0]]++
	}
	return frequentNamespacePrefixes(prefixCounts), nil
}

func learnNamespaceLeaves(seeds []parsedName) map[string][][]string {
	type namespaceGroup struct {
		seeds  int
		leaves map[string]int
	}

	groups := make(map[string]*namespaceGroup)
	for _, seed := range seeds {
		if !seed.expand || len(seed.words) < 2 {
			continue
		}
		namespace := seed.words[0]
		if _, semantic := rulesByToken[namespace]; semantic {
			continue
		}
		if _, known := knownPrefixes[namespace]; known {
			continue
		}
		group := groups[namespace]
		if group == nil {
			group = &namespaceGroup{leaves: make(map[string]int)}
			groups[namespace] = group
		}
		group.seeds++
		group.leaves[seed.words[len(seed.words)-1]]++
	}

	result := make(map[string][][]string)
	for namespace, group := range groups {
		if group.seeds < 2 || len(group.leaves) < 2 {
			continue
		}
		result[namespace] = sortedLeafWords(group.leaves, 8)
	}
	return result
}

func sortedLeafWords(counts map[string]int, limit int) [][]string {
	type counted struct {
		value string
		count int
	}
	values := make([]counted, 0, len(counts))
	for value, count := range counts {
		values = append(values, counted{value: value, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count != values[j].count {
			return values[i].count > values[j].count
		}
		return values[i].value < values[j].value
	})
	if len(values) > limit {
		values = values[:limit]
	}

	leaves := make([][]string, 0, len(values))
	for _, value := range values {
		leaves = append(leaves, []string{value.value})
	}
	return leaves
}

func frequentNamespacePrefixes(counts map[string]int) [][]string {
	type counted struct {
		value string
		count int
	}
	var values []counted
	for value, count := range counts {
		if count < 2 {
			continue
		}
		if _, semantic := rulesByToken[value]; semantic {
			continue
		}
		if _, known := knownPrefixes[value]; known {
			continue
		}
		values = append(values, counted{value: value, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count != values[j].count {
			return values[i].count > values[j].count
		}
		return values[i].value < values[j].value
	})
	if len(values) > 4 {
		values = values[:4]
	}
	result := make([][]string, 0, len(values))
	for _, value := range values {
		result = append(result, []string{value.value})
	}
	return result
}

func (set *orderedSet) add(value string) {
	if value == "" || len(value) > 64 || len(set.values) >= set.limit {
		return
	}
	if _, exists := set.seen[value]; exists {
		return
	}
	set.seen[value] = struct{}{}
	set.values = append(set.values, value)
}
