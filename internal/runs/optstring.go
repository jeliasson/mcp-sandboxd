package runs

import (
	"encoding/json"
)

// OptionalString unmarshals from a JSON string, null, or boolean false.
// This is to tolerate clients that send `shell: false` instead of omitting it.
type OptionalString struct {
	Value string
	Set   bool
}

func (s *OptionalString) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		s.Value = ""
		s.Set = false
		return nil
	}

	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		s.Value = str
		s.Set = true
		return nil
	}

	var bl bool
	if err := json.Unmarshal(b, &bl); err == nil {
		// Treat false as "not set".
		s.Value = ""
		s.Set = false
		return nil
	}

	return json.Unmarshal(b, &str)
}

func (s OptionalString) String() string {
	return s.Value
}
