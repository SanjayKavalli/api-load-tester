package generator

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBuildBodyEmbedsPDFDataURL(t *testing.T) {
	root := filepath.Join("..", "..")
	template := map[string]any{
		"first_name":   "LOAD {{random_letters:8}}",
		"last_name":    "LoadTest",
		"birth_name":   "BIRTH {{random_letters:8}}",
		"dob":          "1995-04-12",
		"gender":       "male",
		"phone_number": "{{random_int:14}}",
		"ssn_passport": "{{ssn}}",
		"document_data": []any{
			map[string]any{
				"doc_data": "data:application/pdf;base64,{{file_base64:../fixtures/sample-result.pdf}}",
			},
		},
	}

	body, err := BuildBody(template, nil, false, filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("BuildBody: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse generated body: %v", err)
	}
	if _, exists := payload["ssn_passport"]; !exists {
		t.Fatal("generated body is missing ssn_passport")
	}
	for _, field := range []string{"first_name", "last_name", "birth_name"} {
		value := payload[field].(string)
		if !regexp.MustCompile(`^[A-Za-z ]+$`).MatchString(value) {
			t.Errorf("%s = %q, want alphabetic characters and spaces only", field, value)
		}
	}
	phone := payload["phone_number"].(string)
	if !regexp.MustCompile(`^[0-9]{1,14}$`).MatchString(phone) {
		t.Errorf("phone_number = %q, want at most 14 digits", phone)
	}
	ssn := payload["ssn_passport"].(string)
	if !regexp.MustCompile(`^[12]9504[0-9]{10}$`).MatchString(ssn) {
		t.Errorf("ssn_passport = %q, want a DOB/gender-compatible SSN", ssn)
	}

	documents, ok := payload["document_data"].([]any)
	if !ok || len(documents) != 1 {
		t.Fatalf("document_data = %#v, want one document", payload["document_data"])
	}
	document, ok := documents[0].(map[string]any)
	if !ok {
		t.Fatalf("document = %#v, want object", documents[0])
	}
	docData, ok := document["doc_data"].(string)
	if !ok {
		t.Fatalf("doc_data = %#v, want string", document["doc_data"])
	}

	const prefix = "data:application/pdf;base64,"
	if strings.Count(docData, prefix) != 1 || !strings.HasPrefix(docData, prefix) {
		t.Fatal("doc_data must contain exactly one PDF data URL prefix")
	}
	pdf, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(docData, prefix))
	if err != nil {
		t.Fatalf("decode doc_data: %v", err)
	}
	if len(pdf) < 4 || string(pdf[:4]) != "%PDF" {
		t.Fatal("decoded doc_data is not a PDF")
	}
}
