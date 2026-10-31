package generator

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var sequence atomic.Uint64

// BuildBody copies the template, optionally overlays dataset values onto keys
// that already exist in the template, resolves supported placeholders, and
// returns the resulting JSON bytes.
//
// Dataset overlay rule:
//   - Only keys already present in the request template can be replaced.
//   - Extra keys in a dataset record are ignored.
//   - Nested JSON objects are overlaid recursively.
func BuildBody(template map[string]any, datasetRecord map[string]any, overlay bool, baseDir string) ([]byte, error) {
	return BuildBodyWithRules(template, datasetRecord, overlay, baseDir, nil)
}

func BuildBodyWithRules(template map[string]any, datasetRecord map[string]any, overlay bool, baseDir string, rules []FieldRule) ([]byte, error) {
	return BuildBodyWithSession(template, datasetRecord, overlay, baseDir, rules, nil)
}

func BuildBodyWithSession(template map[string]any, datasetRecord map[string]any, overlay bool, baseDir string, rules []FieldRule, session *Session) ([]byte, error) {
	body := deepCopyMap(template)
	if overlay && datasetRecord != nil {
		overlayExistingKeys(body, datasetRecord)
	}

	n := sequence.Add(1)
	if session != nil {
		n = session.counter.Add(1)
	}
	if err := applyRules(body, rules, n, session); err != nil {
		return nil, err
	}
	resolved, err := resolveValue(body, baseDir, n)
	if err != nil {
		return nil, err
	}
	resolvedBody, err := json.Marshal(resolved)
	if err != nil {
		return nil, err
	}
	if err := ValidateNumbers(resolved); err != nil {
		return nil, err
	}

	return resolvedBody, nil
}

func overlayExistingKeys(target, source map[string]any) {
	for key, targetValue := range target {
		sourceValue, exists := source[key]
		if !exists {
			continue
		}

		targetMap, targetIsMap := targetValue.(map[string]any)
		sourceMap, sourceIsMap := sourceValue.(map[string]any)
		if targetIsMap && sourceIsMap {
			overlayExistingKeys(targetMap, sourceMap)
			continue
		}

		// Replace only keys present in the request template.
		target[key] = deepCopyValue(sourceValue)
	}
}

func deepCopyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = deepCopyValue(v)
	}
	return dst
}

func deepCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCopyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = deepCopyValue(item)
		}
		return out
	default:
		return x
	}
}

var placeholderRE = regexp.MustCompile(`\{\{(uuid|timestamp|sequence|random_int:[0-9]+|random_string:[0-9]+|random_letters:[0-9]+|file_base64:[^{}]+)\}\}`)
var generatedSSNRE = regexp.MustCompile(`^\{\{ssn\}\}$`)

func resolveValue(v any, baseDir string, seq uint64) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if ssn, ok := x["ssn_passport"].(string); ok && generatedSSNRE.MatchString(ssn) {
			dob, dobOK := x["dob"].(string)
			gender, genderOK := x["gender"].(string)
			if !dobOK || !genderOK {
				return nil, fmt.Errorf("ssn placeholder requires dob and gender fields")
			}
			generated, err := generateSSN(dob, gender)
			if err != nil {
				return nil, err
			}
			x["ssn_passport"] = generated
		}
		for k, child := range x {
			resolved, err := resolveValue(child, baseDir, seq)
			if err != nil {
				return nil, err
			}
			x[k] = resolved
		}
		return x, nil
	case []any:
		for i, child := range x {
			resolved, err := resolveValue(child, baseDir, seq)
			if err != nil {
				return nil, err
			}
			x[i] = resolved
		}
		return x, nil
	case string:
		return resolveString(x, baseDir, seq)
	default:
		return v, nil
	}
}

func resolveString(input, baseDir string, seq uint64) (string, error) {
	matches := placeholderRE.FindAllStringSubmatch(input, -1)
	out := input
	for _, match := range matches {
		token := match[0]
		spec := match[1]
		var replacement string
		var err error

		switch {
		case spec == "uuid":
			replacement, err = newUUID()
		case spec == "timestamp":
			replacement = time.Now().UTC().Format(time.RFC3339Nano)
		case spec == "sequence":
			replacement = fmt.Sprintf("%d", seq)
		case strings.HasPrefix(spec, "random_int:"):
			var n int
			fmt.Sscanf(strings.TrimPrefix(spec, "random_int:"), "%d", &n)
			replacement, err = randomDigits(n)
		case strings.HasPrefix(spec, "random_string:"):
			var n int
			fmt.Sscanf(strings.TrimPrefix(spec, "random_string:"), "%d", &n)
			replacement, err = randomString(n)
		case strings.HasPrefix(spec, "random_letters:"):
			var n int
			fmt.Sscanf(strings.TrimPrefix(spec, "random_letters:"), "%d", &n)
			replacement, err = randomLetters(n)
		case strings.HasPrefix(spec, "file_base64:"):
			path := strings.TrimPrefix(spec, "file_base64:")
			if !filepath.IsAbs(path) {
				path = filepath.Join(baseDir, path)
			}
			var bytes []byte
			bytes, err = os.ReadFile(path)
			if err == nil {
				replacement = base64Encode(bytes)
			}
		}
		if err != nil {
			return "", fmt.Errorf("placeholder %s: %w", token, err)
		}
		out = strings.Replace(out, token, replacement, 1)
	}
	return out, nil
}

func randomDigits(n int) (string, error) {
	if n < 1 || n > 100 {
		return "", fmt.Errorf("random_int length must be 1..100")
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + d.Int64()))
	}
	return b.String(), nil
}

func randomString(n int) (string, error) {
	if n < 1 || n > 100 {
		return "", fmt.Errorf("random_string length must be 1..100")
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[v.Int64()])
	}
	return b.String(), nil
}

func randomLetters(n int) (string, error) {
	if n < 1 || n > 100 {
		return "", fmt.Errorf("random_letters length must be 1..100")
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var b strings.Builder
	for i := 0; i < n; i++ {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[index.Int64()])
	}
	return b.String(), nil
}

func generateSSN(dateOfBirth, gender string) (string, error) {
	dob, err := time.Parse("2006-01-02", dateOfBirth)
	if err != nil {
		return "", fmt.Errorf("ssn placeholder has invalid dob %q: %w", dateOfBirth, err)
	}

	firstDigit := "1"
	switch strings.ToLower(strings.TrimSpace(gender)) {
	case "female":
		firstDigit = "2"
	case "male", "other", "others":
	default:
		return "", fmt.Errorf("ssn placeholder has unsupported gender %q", gender)
	}

	rest, err := randomDigits(10)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d%02d%s", firstDigit, dob.Year()%100, int(dob.Month()), rest), nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(b[:])
	return hexed[:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:], nil
}

func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		n = uint32(data[i]) << 16
		if remaining > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if remaining > 2 {
			n |= uint32(data[i+2])
		}
		out.WriteByte(alphabet[(n>>18)&63])
		out.WriteByte(alphabet[(n>>12)&63])
		if remaining > 1 {
			out.WriteByte(alphabet[(n>>6)&63])
		} else {
			out.WriteByte('=')
		}
		if remaining > 2 {
			out.WriteByte(alphabet[n&63])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}
