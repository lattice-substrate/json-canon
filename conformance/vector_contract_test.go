package conformance_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVectorIntentRequired_CONF_VEC_001(t *testing.T) {
	record := vectorRecord{
		file: "fixture.jsonl",
		line: 1,
		raw: map[string]json.RawMessage{
			"id":        json.RawMessage(`"case-1"`),
			"mode":      json.RawMessage(`"canonicalize"`),
			"want_exit": json.RawMessage(`0`),
		},
		spec: vectorCase{
			ID:       "case-1",
			Mode:     "canonicalize",
			WantExit: 0,
		},
	}

	if err := validateVectorRequiredFields(record); err == nil || !strings.Contains(err.Error(), `"intent"`) {
		t.Fatalf("expected missing intent error, got %v", err)
	}

	record.raw["intent"] = json.RawMessage(`"positive"`)
	record.spec.Intent = vectorIntentPositive
	if err := validateVectorRequiredFields(record); err != nil {
		t.Fatalf("validateVectorRequiredFields positive case: %v", err)
	}
}

func TestVectorIntentEnum_CONF_VEC_002(t *testing.T) {
	if err := validateVectorIntentEnum(vectorCase{Intent: "chaotic"}); err == nil {
		t.Fatal("expected invalid intent rejection")
	}
	for _, intent := range []string{vectorIntentPositive, vectorIntentNegative, vectorIntentAdversarial} {
		if err := validateVectorIntentEnum(vectorCase{Intent: intent}); err != nil {
			t.Fatalf("validateVectorIntentEnum(%q): %v", intent, err)
		}
	}
}

func TestVectorPositiveContract_CONF_VEC_003(t *testing.T) {
	valid := vectorCase{
		ID:         "case-1",
		Intent:     vectorIntentPositive,
		Mode:       "verify",
		WantStderr: stringPtr("ok\n"),
		WantExit:   0,
	}
	if err := validateVectorIntentContract(valid); err != nil {
		t.Fatalf("validateVectorIntentContract positive case: %v", err)
	}

	invalidExit := valid
	invalidExit.WantExit = 2
	if err := validateVectorIntentContract(invalidExit); err == nil || !strings.Contains(err.Error(), "exit 0") {
		t.Fatalf("expected positive exit constraint, got %v", err)
	}

	missingAssertion := valid
	missingAssertion.WantStderr = nil
	if err := validateVectorIntentContract(missingAssertion); err == nil || !strings.Contains(err.Error(), "assert exact stdout or exact stderr") {
		t.Fatalf("expected positive assertion requirement, got %v", err)
	}
}

func TestVectorNegativeContract_CONF_VEC_004(t *testing.T) {
	valid := vectorCase{
		ID:                 "case-1",
		Intent:             vectorIntentNegative,
		Mode:               "verify",
		Input:              "{\"a\":1}",
		WantStderrContains: stringPtr("NOT_CANONICAL"),
		WantExit:           2,
	}
	if err := validateVectorIntentContract(valid); err != nil {
		t.Fatalf("validateVectorIntentContract negative case: %v", err)
	}

	invalidExit := valid
	invalidExit.WantExit = 0
	if err := validateVectorIntentContract(invalidExit); err == nil || !strings.Contains(err.Error(), "fail closed") {
		t.Fatalf("expected negative fail-closed rejection, got %v", err)
	}

	missingDiagnostic := valid
	missingDiagnostic.WantStderrContains = nil
	if err := validateVectorIntentContract(missingDiagnostic); err == nil || !strings.Contains(err.Error(), "stderr evidence") {
		t.Fatalf("expected negative diagnostic requirement, got %v", err)
	}
}

func TestVectorAdversarialContract_CONF_VEC_005(t *testing.T) {
	valid := vectorCase{
		ID:                 "case-1",
		Intent:             vectorIntentAdversarial,
		Mode:               "canonicalize",
		Input:              "{\"a\":1,\"a\":2}",
		WantStderrContains: stringPtr("DUPLICATE_KEY"),
		WantExit:           2,
	}
	if err := validateVectorIntentContract(valid); err != nil {
		t.Fatalf("validateVectorIntentContract adversarial case: %v", err)
	}

	invalidSignal := valid
	invalidSignal.WantStderrContains = stringPtr("INVALID_GRAMMAR")
	if err := validateVectorIntentContract(invalidSignal); err == nil || !strings.Contains(err.Error(), "root-cause or byte-offset diagnostics") {
		t.Fatalf("expected adversarial diagnostic rejection, got %v", err)
	}

	invalidExit := valid
	invalidExit.WantExit = 0
	if err := validateVectorIntentContract(invalidExit); err == nil || !strings.Contains(err.Error(), "fail closed") {
		t.Fatalf("expected adversarial fail-closed rejection, got %v", err)
	}
}

func TestVectorPositiveCoverage_CONF_VEC_006(t *testing.T) {
	records := []vectorRecord{
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandVerify}},
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentAdversarial, Mode: vectorCommandVerify}},
	}
	if missing := missingVectorIntentCoverage(records, vectorIntentPositive); len(missing) != 0 {
		t.Fatalf("unexpected missing positive coverage: %v", missing)
	}

	missing := missingVectorIntentCoverage([]vectorRecord{
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandVerify}},
	}, vectorIntentPositive)
	if len(missing) != 1 || missing[0] != vectorCommandVerify {
		t.Fatalf("expected verify positive gap, got %v", missing)
	}
}

func TestVectorNegativeCoverage_CONF_VEC_007(t *testing.T) {
	records := []vectorRecord{
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandVerify}},
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentAdversarial, Mode: vectorCommandVerify}},
	}
	if missing := missingVectorIntentCoverage(records, vectorIntentNegative); len(missing) != 0 {
		t.Fatalf("unexpected missing negative coverage: %v", missing)
	}

	missing := missingVectorIntentCoverage([]vectorRecord{
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandVerify}},
	}, vectorIntentNegative)
	if len(missing) != 1 || missing[0] != vectorCommandVerify {
		t.Fatalf("expected verify negative gap, got %v", missing)
	}
}

func TestVectorAdversarialCoverage_CONF_VEC_008(t *testing.T) {
	records := []vectorRecord{
		{spec: vectorCase{Intent: vectorIntentAdversarial, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentAdversarial, Mode: vectorCommandVerify}},
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentNegative, Mode: vectorCommandVerify}},
	}
	if missing := missingVectorIntentCoverage(records, vectorIntentAdversarial); len(missing) != 0 {
		t.Fatalf("unexpected missing adversarial coverage: %v", missing)
	}

	missing := missingVectorIntentCoverage([]vectorRecord{
		{spec: vectorCase{Intent: vectorIntentAdversarial, Mode: vectorCommandCanonicalize}},
		{spec: vectorCase{Intent: vectorIntentPositive, Mode: vectorCommandVerify}},
	}, vectorIntentAdversarial)
	if len(missing) != 1 || missing[0] != vectorCommandVerify {
		t.Fatalf("expected verify adversarial gap, got %v", missing)
	}
}

func stringPtr(value string) *string {
	return &value
}
