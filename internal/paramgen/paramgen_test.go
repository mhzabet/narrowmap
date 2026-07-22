package paramgen

import (
	"reflect"
	"sort"
	"testing"
)

func TestGenerateIsDeterministicDeduplicatedAndStyleAware(t *testing.T) {
	inputs := []string{
		"user_id",
		"user_id",
		"accountId",
		"redirect-url",
	}

	first := Generate(inputs, Options{})
	second := Generate(inputs, Options{})

	if !reflect.DeepEqual(first, second) {
		t.Fatal("generation is not deterministic")
	}
	if first.Accepted != 3 || first.Duplicates != 1 || first.Rejected != 0 {
		t.Fatalf("unexpected input stats: %+v", first)
	}
	if !sort.StringsAreSorted(first.Values) {
		t.Fatal("generated values are not sorted")
	}
	for _, expected := range []string{
		"user_id",
		"user_uuid",
		"userUuid",
		"accountId",
		"accountUuid",
		"redirect-url",
		"redirect-path",
	} {
		if !contains(first.Values, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, rejected := range []string{"user_id_id", "accountIdId", "redirect-url-url"} {
		if contains(first.Values, rejected) {
			t.Errorf("duplicate affix candidate %q was generated", rejected)
		}
	}
}

func TestGenerateRejectsMalformedAndRuntimeNoise(t *testing.T) {
	result := Generate([]string{
		"email",
		"q",
		"useEffect",
		"ERR_BAD_REQUEST",
		"https://target.example/?id=1",
		"user_id=7",
		"2fa",
		"aaaaaaaa",
		"data",
	}, Options{})

	if result.Accepted != 3 || result.Rejected != 6 {
		t.Fatalf("unexpected input stats: %+v", result)
	}
	for _, expected := range []string{"data", "email", "q"} {
		if !contains(result.Values, expected) {
			t.Errorf("missing valid seed %q", expected)
		}
	}
	for _, rejected := range []string{"useEffect", "ERR_BAD_REQUEST", "user_id=7"} {
		if contains(result.Values, rejected) {
			t.Errorf("invalid seed %q was retained", rejected)
		}
	}
	if contains(result.Values, "data_id") || contains(result.Values, "current_data") {
		t.Fatal("a low-signal observed seed was expanded into noise")
	}
}

func TestGenerateUsesCustomAffixesAndHonorsPerSeedLimit(t *testing.T) {
	result := Generate([]string{"user_id"}, Options{
		Prefixes:     []string{"internal", "mobile-app"},
		Suffixes:     []string{"hash", "checksum"},
		PerSeedLimit: 16,
	})

	for _, expected := range []string{
		"internal_user_id",
		"mobile_app_user_id",
		"user_checksum",
		"user_hash",
	} {
		if !contains(result.Values, expected) {
			t.Errorf("missing custom-affix candidate %q in %v", expected, result.Values)
		}
	}

	limited := Generate([]string{"user_id"}, Options{PerSeedLimit: 3})
	if len(limited.Values) > 3 {
		t.Fatalf("per-seed limit was exceeded: %v", limited.Values)
	}
	if !contains(limited.Values, "user_id") {
		t.Fatal("the original seed must survive a tight limit")
	}
}

func TestGenerateSupportsBracketParametersWithoutNestedNoise(t *testing.T) {
	result := Generate([]string{"user[id]", "filters[user_id]"}, Options{})

	for _, expected := range []string{
		"user[id]",
		"user_id",
		"user[uuid]",
		"filters[user_id]",
		"filters_user_id",
		"filters_user_uuid",
	} {
		if !contains(result.Values, expected) {
			t.Errorf("missing bracket-style candidate %q in %v", expected, result.Values)
		}
	}
	for _, rejected := range []string{"user[id][id]", "user_id_id"} {
		if contains(result.Values, rejected) {
			t.Errorf("unexpected nested duplicate %q", rejected)
		}
	}
}

func TestGenerateDoesNotApplyCommonObservedSuffixesGlobally(t *testing.T) {
	result := Generate([]string{"user_id", "account_id", "status"}, Options{})
	if contains(result.Values, "status_id") {
		t.Fatal("an observed generic suffix was applied to an unrelated seed")
	}
	for _, expected := range []string{"user_status", "account_status", "payment_status"} {
		if !contains(result.Values, expected) {
			t.Errorf("missing semantic status candidate %q", expected)
		}
	}
}

func TestGenerateKeepsCommonHyphenatedParameters(t *testing.T) {
	result := Generate([]string{"g-recaptcha-response", "h-captcha-response", "x-api-key"}, Options{})
	for _, expected := range []string{"g-recaptcha-response", "h-captcha-response", "x-api-key"} {
		if !contains(result.Values, expected) {
			t.Errorf("missing common parameter %q", expected)
		}
	}
	if contains(result.Values, "g-recaptcha-response-id") {
		t.Fatal("a standardized CAPTCHA field was expanded")
	}
}

func TestGenerateCreatesSeparatorPreservingFuzzTemplates(t *testing.T) {
	result := Generate([]string{
		"test_param",
		"test.param",
		"test-param",
		"testParam",
		"filters[user_id]",
	}, Options{})

	for _, expected := range []string{
		"FUZZ_param", "test_FUZZ",
		"FUZZ.param", "test.FUZZ",
		"FUZZ-param", "test-FUZZ",
		"FUZZParam", "testFUZZ",
		"FUZZ[user_id]", "filters[FUZZ_id]", "filters[user_FUZZ]",
	} {
		if !contains(result.Values, expected) {
			t.Errorf("missing FUZZ template %q in %v", expected, result.Values)
		}
	}
}

func TestGenerateDoesNotFuzzStandardizedCaptchaFields(t *testing.T) {
	result := Generate([]string{"g-recaptcha-response"}, Options{})
	if contains(result.Values, "FUZZ-recaptcha-response") || contains(result.Values, "g-recaptcha-FUZZ") {
		t.Fatalf("standardized CAPTCHA field was expanded into FUZZ templates: %v", result.Values)
	}
}

func TestGenerateRejectsMalformedBracketNames(t *testing.T) {
	result := Generate([]string{"user[id]tail", "user[]", "[user]"}, Options{})
	if result.Accepted != 0 || result.Rejected != 3 {
		t.Fatalf("malformed bracket names were accepted: %+v", result)
	}
}

func TestGenerateReplacesStableCompoundLeaves(t *testing.T) {
	result := Generate([]string{
		"yahoo_home_ui",
		"yahoo_profile_url",
	}, Options{})

	for _, expected := range []string{
		"yahoo_home_ui",
		"yahoo_home_redirect",
		"yahoo_home_callback",
		"yahoo_home_url",
		"yahoo_home_status",
		"yahoo_profile_redirect",
		"yahoo_profile_callback",
	} {
		if !contains(result.Values, expected) {
			t.Errorf("missing terminal-word substitution %q", expected)
		}
	}
	for _, noisy := range []string{
		"yahoo_home_ui_redirect",
		"current_yahoo_home_ui",
		"yahoo_profile_url_redirect",
	} {
		if contains(result.Values, noisy) {
			t.Errorf("terminal word was appended instead of replaced: %q", noisy)
		}
	}
}

func TestGenerateReusesLeavesInsideTargetNamespace(t *testing.T) {
	result := Generate([]string{
		"acme_home_ui",
		"acme_account_experiment",
		"other_home_ui",
	}, Options{})

	for _, expected := range []string{
		"acme_home_experiment",
		"acme_account_ui",
	} {
		if !contains(result.Values, expected) {
			t.Errorf("missing learned namespace mutation %q", expected)
		}
	}
	if contains(result.Values, "other_home_experiment") {
		t.Fatal("a learned leaf crossed into an unrelated namespace")
	}
}

func BenchmarkGenerate(b *testing.B) {
	inputs := []string{
		"user_id", "accountId", "customer-email", "redirect_url",
		"access_token", "page_size", "payment_status", "project[id]",
	}
	for b.Loop() {
		Generate(inputs, Options{})
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
