package model

import (
	"encoding/json"
	"io"
)

type Starred struct {
	Items []*Item `json:"items"`
}

func (s *Starred) FromJSON(reader io.Reader) (*Starred, error) {
	err := json.NewDecoder(reader).Decode(s)
	if err == nil {
		for _, item := range s.Items {
			item.DecodeFields()
		}
	}
	return s, err
}
