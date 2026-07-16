package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

const cliSchemaVersion = "switchyard.cli/v1"

type outputEnvelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Command       string `json:"command"`
	Data          any    `json:"data"`
}

func writeResult(options *rootOptions, command string, data any, human func(io.Writer) error) error {
	if options.jsonl {
		value := reflect.ValueOf(data)
		if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
			return usageError("JSONL_UNSUPPORTED", "--jsonl is supported only for list or streaming commands")
		}
		encoder := json.NewEncoder(options.stdout)
		for index := 0; index < value.Len(); index++ {
			if err := encoder.Encode(outputEnvelope{SchemaVersion: cliSchemaVersion, Command: command, Data: value.Index(index).Interface()}); err != nil {
				return err
			}
		}
		return nil
	}
	if options.json {
		return writePrettyJSON(options.stdout, outputEnvelope{SchemaVersion: cliSchemaVersion, Command: command, Data: data})
	}
	return human(options.stdout)
}

func writePrettyJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeMachineError(writer io.Writer, cliErr *Error) error {
	return json.NewEncoder(writer).Encode(map[string]any{
		"schemaVersion": cliSchemaVersion,
		"error":         map[string]any{"code": cliErr.Code, "message": cliErr.Message, "exitCode": cliErr.ExitCode},
	})
}

func machineRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--jsonl" {
			return true
		}
	}
	return false
}

func humanList(writer io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = len(header)
	}
	for _, row := range rows {
		for index, cell := range row {
			if len(cell) > widths[index] {
				widths[index] = len(cell)
			}
		}
	}
	printRow := func(row []string) error {
		for index, cell := range row {
			if index == len(row)-1 {
				if _, err := fmt.Fprint(writer, cell); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(writer, "%-*s  ", widths[index], cell); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(writer)
		return err
	}
	if err := printRow(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := printRow(row); err != nil {
			return err
		}
	}
	return nil
}
