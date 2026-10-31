package generator

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
)

type FieldRule struct {
	Path       []string `json:"path"`
	Regex      string   `json:"regex,omitempty"`
	Expression string   `json:"expression,omitempty"`
	Unique     bool     `json:"unique,omitempty"`
}

func ValidateRules(rules []FieldRule) error {
	for index, rule := range rules {
		if len(rule.Path) == 0 || (rule.Regex == "") == (rule.Expression == "") {
			return fmt.Errorf("each field rule needs a path and exactly one regex or expression")
		}
		for _, earlier := range rules[:index] {
			limit := min(len(rule.Path), len(earlier.Path))
			overlaps := true
			for part := 0; part < limit; part++ {
				if rule.Path[part] != earlier.Path[part] {
					overlaps = false
					break
				}
			}
			if overlaps {
				return fmt.Errorf("field rules have duplicate or overlapping paths: %v and %v", earlier.Path, rule.Path)
			}
		}
	}
	return nil
}

func applyRules(body map[string]any, rules []FieldRule, seq uint64, session *Session) error {
	if err := ValidateRules(rules); err != nil {
		return err
	}
	if session == nil {
		var err error
		session, err = NewSession(nil, false)
		if err != nil {
			return err
		}
	}
	for _, rule := range rules {
		if len(rule.Path) == 0 || (rule.Regex == "") == (rule.Expression == "") {
			return fmt.Errorf("each field rule needs a path and exactly one regex or expression")
		}
		if session.ReuseExisting && rule.Unique {
			continue
		}
		var value any
		var err error
		for attempt := 0; attempt < 128; attempt++ {
			if rule.Regex != "" {
				value, err = GenerateRegex(rule.Regex)
			} else {
				env := map[string]any{"body": body, "sequence": seq, "run_id": session.RunID, "ssn": generateSSN}
				var programResult any
				program, compileErr := expr.Compile(rule.Expression, expr.Env(env), expr.MaxNodes(1000))
				if compileErr != nil {
					err = compileErr
				} else {
					programResult, err = expr.Run(program, env)
				}
				value = programResult
				if err == nil {
					encoded, encodeErr := json.Marshal(value)
					if encodeErr != nil {
						err = encodeErr
					} else {
						err = json.Unmarshal(encoded, &value)
					}
				}
			}
			if err != nil || !rule.Unique {
				break
			}
			var reserved bool
			reserved, err = session.store.reserve(rule.Path, value)
			if err != nil || reserved {
				break
			}
			if attempt == 127 {
				err = ErrUniqueUnavailable
			}
		}
		if err != nil {
			if rule.Unique {
				return fmt.Errorf("%w: field %v: %w", ErrUniqueGeneration, rule.Path, err)
			}
			return fmt.Errorf("field %v: %w", rule.Path, err)
		}
		if containsFileReference(value) {
			return fmt.Errorf("field rules cannot generate server file references")
		}
		var parent any = body
		for index, key := range rule.Path {
			last := index == len(rule.Path)-1
			switch target := parent.(type) {
			case map[string]any:
				child, exists := target[key]
				if !exists {
					return fmt.Errorf("field path %v does not exist", rule.Path)
				}
				if last {
					target[key] = value
				} else {
					parent = child
				}
			case []any:
				position, parseErr := strconv.Atoi(key)
				if parseErr != nil || position < 0 || position >= len(target) || strconv.Itoa(position) != key {
					return fmt.Errorf("invalid array path %v", rule.Path)
				}
				if last {
					target[position] = value
				} else {
					parent = target[position]
				}
			default:
				return fmt.Errorf("invalid field path %v", rule.Path)
			}
		}
	}
	return nil
}

func containsFileReference(value any) bool {
	switch item := value.(type) {
	case string:
		return strings.Contains(item, "file_base64:")
	case map[string]any:
		for _, child := range item {
			if containsFileReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if containsFileReference(child) {
				return true
			}
		}
	}
	return false
}

func GenerateRegex(pattern string) (string, error) {
	if len(pattern) > 2048 {
		return "", fmt.Errorf("regex exceeds 2048 bytes")
	}
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", err
	}
	budget := 4096
	value, err := regexValue(tree, &budget)
	if err != nil {
		return "", err
	}
	compiled, err := regexp.Compile("\\A(?:" + pattern + ")\\z")
	if err != nil {
		return "", err
	}
	if !compiled.MatchString(value) {
		return "", fmt.Errorf("regex uses unsupported positional assertions; use anchors only at the edges")
	}
	return value, nil
}

func randomBelow(limit int) int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		panic(err)
	}
	return int(value.Int64())
}

func regexValue(node *syntax.Regexp, budget *int) (string, error) {
	*budget--
	if *budget < 0 {
		return "", fmt.Errorf("regex generation exceeds 4096 steps")
	}
	switch node.Op {
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		return "", nil
	case syntax.OpLiteral:
		return string(node.Rune), nil
	case syntax.OpCharClass:
		count := 0
		for index := 0; index < len(node.Rune); index += 2 {
			count += int(node.Rune[index+1]-node.Rune[index]) + 1
		}
		choice := randomBelow(count)
		for index := 0; index < len(node.Rune); index += 2 {
			size := int(node.Rune[index+1]-node.Rune[index]) + 1
			if choice < size {
				return string(node.Rune[index] + rune(choice)), nil
			}
			choice -= size
		}
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return string(rune('a' + randomBelow(26))), nil
	case syntax.OpCapture:
		return regexValue(node.Sub[0], budget)
	case syntax.OpAlternate:
		return regexValue(node.Sub[randomBelow(len(node.Sub))], budget)
	case syntax.OpConcat:
		output := ""
		for _, child := range node.Sub {
			part, err := regexValue(child, budget)
			if err != nil {
				return "", err
			}
			output += part
		}
		return output, nil
	case syntax.OpQuest, syntax.OpStar, syntax.OpPlus, syntax.OpRepeat:
		minimum, maximum := node.Min, node.Max
		switch node.Op {
		case syntax.OpQuest:
			minimum, maximum = 0, 1
		case syntax.OpStar:
			minimum, maximum = 0, 12
		case syntax.OpPlus:
			minimum, maximum = 1, 12
		}
		if maximum < 0 {
			maximum = minimum + 12
		}
		count := minimum + randomBelow(maximum-minimum+1)
		output := ""
		for index := 0; index < count; index++ {
			part, err := regexValue(node.Sub[0], budget)
			if err != nil {
				return "", err
			}
			output += part
		}
		return output, nil
	}
	return "", fmt.Errorf("unsupported regex construct %s", node.Op)
}
