package model

import (
	"encoding/json"
	"io"
)

type Starred struct {
	Items []*Item `json:"items"`
}

func (s *Starred) FromJSON(reader io.Reader) (*Starred, error) {
	return s, json.NewDecoder(reader).Decode(s)
}
