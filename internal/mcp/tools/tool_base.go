package tools

import (
	"encoding/json"
	"fmt"
)

// ParseParams parses and validates generic parameters from interface{} to a target struct.
// This eliminates the duplicate marshal/unmarshal pattern found across all tool handlers.
//
// Usage:
//
//	var params MyParams
//	if err := ParseParams(req.Params, &params); err != nil {
//	    return errorResponse(err)
//	}
func ParseParams(params interface{}, target interface{}) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	return nil
}
