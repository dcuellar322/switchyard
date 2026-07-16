package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	invopop "github.com/invopop/jsonschema"
	validator "github.com/santhosh-tekuri/jsonschema/v6"
)

// ProposalOutputSchema returns the canonical provider response schema.
func ProposalOutputSchema() (json.RawMessage, error) {
	reflector := invopop.Reflector{AllowAdditionalProperties: false, RequiredFromJSONSchemaTags: true}
	schema := reflector.Reflect(&ProposalOutput{})
	schema.ID = invopop.ID("https://switchyard.dev/schema/ai-proposal.v1alpha1.json")
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encode provider output schema: %w", err)
	}
	return encoded, nil
}

func decodeProviderOutput(raw json.RawMessage, limit int64) (ProposalOutput, error) {
	if int64(len(raw)) > limit {
		return ProposalOutput{}, fmt.Errorf("%w: response exceeds %d bytes", ErrProviderOutput, limit)
	}
	schema, err := ProposalOutputSchema()
	if err != nil {
		return ProposalOutput{}, err
	}
	compiler := validator.NewCompiler()
	var schemaDocument any
	if err := json.Unmarshal(schema, &schemaDocument); err != nil {
		return ProposalOutput{}, fmt.Errorf("decode provider schema: %w", err)
	}
	if err := compiler.AddResource("ai-proposal.schema.json", schemaDocument); err != nil {
		return ProposalOutput{}, fmt.Errorf("load provider schema: %w", err)
	}
	compiled, err := compiler.Compile("ai-proposal.schema.json")
	if err != nil {
		return ProposalOutput{}, fmt.Errorf("compile provider schema: %w", err)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return ProposalOutput{}, fmt.Errorf("%w: malformed JSON: %v", ErrProviderOutput, err)
	}
	if err := compiled.Validate(document); err != nil {
		return ProposalOutput{}, fmt.Errorf("%w: schema validation: %v", ErrProviderOutput, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var output ProposalOutput
	if err := decoder.Decode(&output); err != nil {
		return ProposalOutput{}, fmt.Errorf("%w: decode: %v", ErrProviderOutput, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ProposalOutput{}, fmt.Errorf("%w: %v", ErrProviderOutput, err)
	}
	return output, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("multiple JSON documents are not allowed")
}
