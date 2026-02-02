package models

import (
	"encoding/json"
)

// Record represents a single JSON object in the collection
type Record struct {
	ID   int             `json:"id"`
	Data json.RawMessage `json:"-"` // Data holds the rest of the JSON object
}

// UnmarshalJSON custom unmarshaling to handle dynamic data
func (r *Record) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	idFloat, ok := raw["id"].(float64)
	if !ok {
		return json.Unmarshal(data, &r.ID) // if id is not number, error
	}
	r.ID = int(idFloat)
	delete(raw, "id")
	r.Data, _ = json.Marshal(raw)
	return nil
}

// MarshalJSON custom marshaling to include data
func (r Record) MarshalJSON() ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(r.Data, &raw); err != nil {
		return nil, err
	}
	raw["id"] = r.ID
	return json.Marshal(raw)
}