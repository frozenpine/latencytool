package ctl

import "encoding/json"

type Result struct {
	Rtn     int
	Message string
	CmdName string
	Values  map[string]any
}

func (r *Result) UnmarshalJSON(v []byte) error {
	data := make(map[string]json.RawMessage)

	if err := json.Unmarshal(v, &data); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Rtn"], &r.Rtn); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Message"], &r.Message); err != nil {
		return err
	}

	if err := json.Unmarshal(data["CmdName"], &r.CmdName); err != nil {
		return err
	}

	values := make(map[string]json.RawMessage)
	r.Values = make(map[string]any)
	if err := json.Unmarshal(data["Values"], &values); err != nil {
		return nil
	}

	for k, v := range values {
		r.Values[k] = v
	}

	return nil
}
