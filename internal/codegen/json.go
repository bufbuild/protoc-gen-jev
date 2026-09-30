package codegen

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/sudorandom/protoc-gen-jev/internal/model"
)

// GenerateJSON writes the language-agnostic .jev.json specification.
func GenerateJSON(gen *protogen.Plugin, file *protogen.File, specs []model.MessageSpec, serviceSpecs []model.ServiceSpec) error {
	for _, spec := range specs {
		jsonFilename := fmt.Sprintf("%s_%s.jev.json", file.GeneratedFilenamePrefix, spec.MessageName)
		jsonFile := gen.NewGeneratedFile(jsonFilename, "")
		data, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode Jev JSON spec for %s: %w", spec.MessageName, err)
		}
		if _, err := jsonFile.Write(data); err != nil {
			return err
		}
	}

	for _, svc := range serviceSpecs {
		jsonFilename := fmt.Sprintf("%s_%s.jev.json", file.GeneratedFilenamePrefix, svc.ServiceName)
		jsonFile := gen.NewGeneratedFile(jsonFilename, "")
		data, err := json.MarshalIndent(svc, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode Jev JSON spec for service %s: %w", svc.ServiceName, err)
		}
		if _, err := jsonFile.Write(data); err != nil {
			return err
		}
	}

	return nil
}
