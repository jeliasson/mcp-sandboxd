package runs

import (
	"encoding/json"
	"strings"
)

// EnvVars accepts either an object map {"K":"V"} or an array ["K=V", ...].
// This is primarily for compatibility with clients that encode env as argv-like.
type EnvVars map[string]string

func (e *EnvVars) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*e = nil
		return nil
	}

	// Try object form first.
	var obj map[string]string
	if err := json.Unmarshal(b, &obj); err == nil {
		*e = obj
		return nil
	}

	// Try array form.
	var arr []string
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}

	m := map[string]string{}
	for _, s := range arr {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		m[k] = v
	}
	*e = m
	return nil
}

func (e EnvVars) Map() map[string]string {
	if e == nil {
		return nil
	}
	return map[string]string(e)
}
