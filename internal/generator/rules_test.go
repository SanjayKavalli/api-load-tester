package generator

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestRulesRejectUniqueOverrides(t *testing.T) {
	for _, rules := range [][]FieldRule{
		{{Path: []string{"id"}, Regex: "[A-Z]{8}", Unique: true}, {Path: []string{"id"}, Expression: `"SAME"`}},
		{{Path: []string{"patient", "id"}, Regex: "[A-Z]{8}", Unique: true}, {Path: []string{"patient"}, Expression: `{"id":"SAME"}`}},
	} {
		if _, err := BuildBodyWithRules(map[string]any{"id": "", "patient": map[string]any{"id": ""}}, nil, false, "", rules); err == nil {
			t.Fatal("accepted an overlapping field rule")
		}
	}
}

func TestDatasetArraysRemainUnchanged(t *testing.T) {
	record := map[string]any{"items": []any{map[string]any{"id": "{{uuid}}"}}}
	template := map[string]any{"items": []any{map[string]any{"id": ""}}}
	first, err := BuildBody(template, record, true, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildBody(template, record, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("dataset UUID was reused")
	}
	if record["items"].([]any)[0].(map[string]any)["id"] != "{{uuid}}" {
		t.Fatal("source dataset mutated")
	}
}

func TestUnsafeIdentifiersAreRejected(t *testing.T) {
	if err := ValidateJSONNumbers([]byte(`{"id":1e99999999}`)); err == nil {
		t.Fatal("out-of-range number accepted")
	}
	if err := ValidateJSONNumbers([]byte(`{"id":9007199254740993}`)); err == nil {
		t.Fatal("unsafe integer accepted")
	}
	if err := ValidateJSONNumbers([]byte(`{"id":"9007199254740993"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBody(map[string]any{"id": float64(9007199254740992)}, nil, false, ""); err == nil {
		t.Fatal("rounded integer was generated")
	}
}

func TestArrayRulePathsMustUseCanonicalIndexes(t *testing.T) {
	for _, index := range []string{"00", "+0", "-0"} {
		_, err := BuildBodyWithRules(map[string]any{"items": []any{map[string]any{"id": ""}}}, nil, false, "", []FieldRule{{Path: []string{"items", index, "id"}, Regex: "A"}})
		if err == nil {
			t.Fatal("array index alias accepted", index)
		}
	}
}

func TestFieldRules(t *testing.T) {
	template := map[string]any{"id": "", "label": "", "items": []any{map[string]any{"code": ""}}}
	rules := []FieldRule{
		{Path: []string{"id"}, Regex: `^PT-[A-Z]{3}[0-9]{4}$`},
		{Path: []string{"label"}, Expression: `body.id + "-active"`},
		{Path: []string{"items", "0", "code"}, Regex: `(A|B){2,4}`},
	}
	for index := 0; index < 30; index++ {
		body, err := BuildBodyWithRules(template, nil, false, "", rules)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(rules[0].Regex).MatchString(result["id"].(string)) {
			t.Fatal(string(body))
		}
		if result["label"] != result["id"].(string)+"-active" {
			t.Fatal(string(body))
		}
	}
	if template["id"] != "" {
		t.Fatal("template mutated")
	}
}

func TestInvalidRules(t *testing.T) {
	for _, rule := range []FieldRule{
		{Path: []string{"id"}, Regex: `[`},
		{Path: []string{"missing"}, Regex: `[A-Z]`},
		{Path: []string{"id"}, Expression: `unknown()`},
		{Path: []string{"id"}, Regex: `\btest\b`},
	} {
		if _, err := BuildBodyWithRules(map[string]any{"id": ""}, nil, false, "", []FieldRule{rule}); err == nil {
			t.Fatalf("accepted %#v", rule)
		}
	}
}

func TestExpressionsReturnIndependentJSONValues(t *testing.T) {
	for _, expression := range []string{`body`, `body.quantity * 3`, `body.quantity >= 2`} {
		body, err := BuildBodyWithRules(map[string]any{"quantity": 2.0, "result": nil}, nil, false, "", []FieldRule{{Path: []string{"result"}, Expression: expression}})
		if err != nil {
			t.Fatalf("%s: %v", expression, err)
		}
		if !json.Valid(body) {
			t.Fatal(string(body))
		}
	}
}
