package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	_ "embed"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const V0URL = "https://cronctl.usoltsev.xyz/v0.json"

var errEmptySchema = errors.New("embedded schema is empty")

//go:embed v0.json
var v0Bytes []byte

func V0() (*jsonschema.Schema, error) {
	b := bytes.TrimSpace(v0Bytes)
	if len(b) == 0 {
		return nil, errEmptySchema
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse embedded schema json: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(V0URL, doc); err != nil {
		return nil, fmt.Errorf("add embedded schema resource: %w", err)
	}
	s, err := c.Compile(V0URL)
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}
	return s, nil
}
