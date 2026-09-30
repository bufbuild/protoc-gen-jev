// protoc-gen-jev is an experimental Protobuf compiler plugin for TypeSafe AI's Jev.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/bufbuild/protoc-gen-jev/internal/codegen"
	"github.com/bufbuild/protoc-gen-jev/internal/model"
	"github.com/bufbuild/protoc-gen-jev/internal/parser"
)

var version = "dev"

func getVersion() string {
	v := version
	if v == "" || v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	if v == "" {
		return "dev"
	}
	return v
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "-v" || arg == "-V" || arg == "version" {
			fmt.Printf("protoc-gen-jev %s\n", getVersion())
			return
		}
	}

	var flags flag.FlagSet
	var targets []string
	addTarget := func(value string) error { targets = append(targets, value); return nil }
	flags.Func("target", "Target language (repeat for multiple targets)", addTarget)
	flags.Func("targets", "Alias for target", addTarget)

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL | pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
		gen.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_2023
		gen.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2024

		val := strings.Join(targets, ",")
		targetOptions, err := model.ParseTargets(val)
		if err != nil {
			return err
		}

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if f.Desc.Package() == "jev.v1" {
				continue
			}

			var specs []model.MessageSpec
			for _, msg := range parser.AllMessages(f.Messages) {
				spec, err := parser.ProcessMessage(msg)
				if err != nil {
					return err
				}
				if len(spec.Questions) > 0 {
					specs = append(specs, spec)
				}
			}

			var serviceSpecs []model.ServiceSpec
			for _, svc := range f.Services {
				serviceSpec, err := parser.ProcessService(svc)
				if err != nil {
					return err
				}
				if len(serviceSpec.Methods) > 0 {
					serviceSpecs = append(serviceSpecs, serviceSpec)
				}
			}

			if len(specs) == 0 && len(serviceSpecs) == 0 {
				continue
			}

			// 1. Language-agnostic JSON spec
			if targetOptions.GenerateJSON {
				if err := codegen.GenerateJSON(gen, f, specs, serviceSpecs); err != nil {
					return err
				}
			}

			// 2. Native Go client
			if targetOptions.GenerateGo {
				if err := codegen.GenerateGo(gen, f, specs, serviceSpecs); err != nil {
					return err
				}
			}

			// 3. TypeScript client
			if targetOptions.GenerateTypeScript {
				if err := codegen.GenerateTypeScript(gen, f, specs, serviceSpecs); err != nil {
					return err
				}
			}

			// 4. Python client
			if targetOptions.GeneratePython {
				if err := codegen.GeneratePython(gen, f, specs, serviceSpecs); err != nil {
					return err
				}
			}
		}

		return nil
	})
}
