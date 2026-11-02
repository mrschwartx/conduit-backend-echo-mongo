package domain

import "encoding/json"

type Raw struct {
	value any
	set   bool
}

func RawOf(value any) Raw {
	return Raw{value: value, set: true}
}

func RawString(value string) Raw {
	return RawOf(value)
}

func (r *Raw) UnmarshalJSON(data []byte) error {
	r.set = true
	r.value = nil
	return json.Unmarshal(data, &r.value)
}

func (r Raw) IsSet() bool {
	return r.set
}

func (r Raw) Value() any {
	return r.value
}

func (r Raw) Text() (string, bool) {
	text, ok := r.value.(string)
	return text, r.set && ok
}
