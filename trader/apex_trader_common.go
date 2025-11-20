package trader

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// APIError define API error when response status is 4xx or 5xx
type APIError struct {
	Code    int64  `json:"retCode"`
	Message string `json:"retMsg"`
}

// Error return error code and message
func (e APIError) Error() string {
	return fmt.Sprintf("<APIError> code=%d, msg=%s", e.Code, e.Message)
}

// IsAPIError check if e is an API error
func IsAPIError(e error) bool {
	var APIError *APIError
	ok := errors.As(e, &APIError)
	return ok
}

func ValidateParams(params map[string]interface{}) error {
	seenKeys := make(map[string]bool)

	for key, value := range params {
		if key == "" {
			return fmt.Errorf("empty key found in parameters")
		}
		if seenKeys[key] {
			return fmt.Errorf("duplicate key found in parameters: %s", key)
		}
		if value == nil {
			return fmt.Errorf("parameter for key '%s' is nil", key)
		}
		seenKeys[key] = true
	}
	return nil
}

func Strval(value interface{}) string {
	var key string
	if value == nil {
		return key
	}

	switch value.(type) {
	case float64:
		ft := value.(float64)
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := value.(float32)
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		it := value.(int)
		key = strconv.Itoa(it)
	case uint:
		it := value.(uint)
		key = strconv.Itoa(int(it))
	case int8:
		it := value.(int8)
		key = strconv.Itoa(int(it))
	case uint8:
		it := value.(uint8)
		key = strconv.Itoa(int(it))
	case int16:
		it := value.(int16)
		key = strconv.Itoa(int(it))
	case uint16:
		it := value.(uint16)
		key = strconv.Itoa(int(it))
	case int32:
		it := value.(int32)
		key = strconv.Itoa(int(it))
	case uint32:
		it := value.(uint32)
		key = strconv.Itoa(int(it))
	case int64:
		it := value.(int64)
		key = strconv.FormatInt(it, 10)
	case uint64:
		it := value.(uint64)
		key = strconv.FormatUint(it, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}

	return key
}

// Stringify converts interface{} to string representation.
func Stringify(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 64)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", t)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t)
	default:
		b, _ := json.Marshal(t)
		s := string(b)
		if s == "null" {
			return ""
		}
		return s
	}
}

// BuildSortedKV builds sorted key-value pairs string.
func BuildSortedKV(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if m[k] == "" {
			continue
		}
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
	}
	return b.String()
}
