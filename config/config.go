package config

import (
	"encoding/json"
	"errors"
	"io"
)

type GHRelCli struct {
	Repository  string `json:"repository"`
	TagName     string `json:"tag_name"`
	AccessToken string `json:"access_token"`
}

func NewFromReader(reader io.Reader) (GHRelCli, error) {
	dec := json.NewDecoder(reader)
	var c GHRelCli
	if err := dec.Decode(&c); err != nil {
		return GHRelCli{}, err
	}

	if c.Repository == "" {
		return GHRelCli{}, errors.New("repository must be set")
	}

	if c.TagName == "" {
		return GHRelCli{}, errors.New("tag_name must be set")
	}
	return c, nil
}
