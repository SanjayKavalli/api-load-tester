package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

func ValidateJSONNumbers(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	return ValidateNumbers(value)
}

func ValidateNumbers(value any) error {
	switch item := value.(type) {
	case json.Number:
		number, err := item.Float64()
		if err != nil {
			return fmt.Errorf("JSON number is outside the supported numeric range; encode identifiers as strings")
		}
		if err := ValidateNumbers(number); err != nil {
			return err
		}
	case float64:
		if math.Trunc(item) == item && math.Abs(item) > 9007199254740991 {
			return fmt.Errorf("integer exceeds safe JSON precision; encode identifiers as strings")
		}
	case map[string]any:
		for _, child := range item {
			if err := ValidateNumbers(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range item {
			if err := ValidateNumbers(child); err != nil {
				return err
			}
		}
	}
	return nil
}
