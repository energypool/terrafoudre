package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tf "github.com/hashicorp/terraform-json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type SourceResource struct {
	Key          string
	Value        string
	Provider     string
	Address      string
	Destinations []DestinationResource
}

type DestinationResource struct {
	Provider string
	Address  string
	State    string
	Mode     string
	Key      string
}

var (
	conf Configuration
)

func main() {
	////////////////
	// CLI Params
	////////////////
	debug := flag.Bool("debug", false, "Set log level to debug")
	confPath := flag.String("config", "test/config-full.yaml", "Path to config")
	sourceState := flag.String("sourceState", "test/states/source.json", "Path to json state that contains source resources")
	destinationStates := flag.String("destinationStates", "test/states/", "Path to json states directory that contains destination resources")
	flag.Parse()

	////////////////
	// Instantiate Pretty Logger
	////////////////
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	////////////////
	// Load conf and states
	////////////////
	conf.LoadConfiguration(*confPath)
	dStates := listDestinationStates(*sourceState, *destinationStates)

	statefile, err := os.ReadFile(*sourceState)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't read state file")
	}
	var state tf.State
	err = state.UnmarshalJSON(statefile)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't read state file")
	}
	log.Debug().Msgf("Source State format version %s", state.FormatVersion)
	log.Debug().Msgf("Source Terraform version %s", state.TerraformVersion)

	////////////////
	// Extract source resources interesting keys
	////////////////
	sourceResources := []SourceResource{}
	sourceResources = append(sourceResources, extractSourceResources(state.Values.RootModule)...)
	for _, child := range state.Values.RootModule.ChildModules {
		sourceResources = append(sourceResources, extractSourceResources(child)...)
	}

	////////////////
	// Find if source resources are in destination resources
	////////////////
	sourceResources = analyzeDestinationStates(dStates, sourceResources)

	////////////////
	// Display Results
	////////////////
	for _, source := range sourceResources {
		if source.Destinations != nil {
			log.Info().Interface("Source", source).Send()
		}
	}
}

// List all states that will be used as destination
func listDestinationStates(source, dir string) []string {
	paths := []string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Error().Err(err).Send()
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") && path != source {
			log.Debug().Str("Found destination state", path).Send()
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Send()
	}
	return paths
}

// Extract all key/value from resources that will be used to search in destination resource
func extractSourceResources(state *tf.StateModule) []SourceResource {
	resources := []SourceResource{}
	for _, resource := range state.Resources {
		if string(resource.Mode) == "managed" {
			for key, value := range resource.AttributeValues {

				// Check if key is a configured primary key
				matchers := getMatchers(resource.Type, resource.ProviderName)
				valid := false
				for _, validKey := range matchers {
					if key == validKey {
						valid = true
					}
				}
				if valueStr, ok := value.(string); ok && valid {
					resources = append(resources, SourceResource{
						Key:      key,
						Value:    valueStr,
						Provider: resource.ProviderName,
						Address:  resource.Address,
					})
				}
			}
		}
	}
	return resources
}

// Try to find source resource references in destination states
func analyzeDestinationStates(destinationStates []string, sourceResources []SourceResource) []SourceResource {
	sResources := []SourceResource{}
	for _, statefile := range destinationStates {

		// Load the state object from file
		var st tf.State
		file, err := os.ReadFile(statefile)
		if err != nil {
			log.Error().Err(err).Msgf("Can't read state file %s", statefile)
			continue
		}
		err = st.UnmarshalJSON(file)
		if err != nil {
			log.Error().Err(err).Msgf("Can't read state file %s", statefile)
			continue
		}

		// Add all destinations in source resources
		for _, source := range sourceResources {
			source.Destinations = append(source.Destinations, extractDestinationsResourcesThatUseSourceResource(statefile, st.Values.RootModule, source)...)
			for _, child := range st.Values.RootModule.ChildModules {
				source.Destinations = append(source.Destinations, extractDestinationsResourcesThatUseSourceResource(statefile, child, source)...)
			}
			sResources = append(sResources, source)
		}
	}
	return sResources
}

// Check all values from a given destination resource
func extractDestinationsResourcesThatUseSourceResource(statefile string, state *tf.StateModule, source SourceResource) []DestinationResource {
	resourceMatch := []DestinationResource{}
	for _, resource := range state.Resources {
		for key, value := range resource.AttributeValues {

			// Check if resource key is banned
			matchers := getMatchers(resource.Type, resource.ProviderName)
			banned := false
			for _, matcher := range matchers {
				if key == matcher {
					banned = true
				}
			}
			if string(resource.Mode) == "data" {
				banned = false
			}
			if banned {
				continue
			}

			// Define value type
			switch v := value.(type) {
			case string:
				// log.Debug().Interface("value", v).Send()
				if v == source.Value {
					resourceMatch = append(resourceMatch, DestinationResource{
						Provider: resource.ProviderName,
						Address:  resource.Address,
						State:    statefile,
						Mode:     string(resource.Mode),
						Key:      key,
					})
				}
			case []interface{}:
				for _, vv := range v {
					switch vvv := vv.(type) {
					case string:
						if vvv == source.Value {
							resourceMatch = append(resourceMatch, DestinationResource{
								Provider: resource.ProviderName,
								Address:  resource.Address,
								State:    statefile,
								Mode:     string(resource.Mode),
								Key:      key,
							})
						}
					}
				}
			case map[string]interface{}:
				if ok, k := interfaceTreeMatch(v, source.Value); ok {
					resourceMatch = append(resourceMatch, DestinationResource{
						Provider: resource.ProviderName,
						Address:  resource.Address,
						State:    statefile,
						Mode:     string(resource.Mode),
						Key:      fmt.Sprintf("%s.%s", key, k),
					})
				}
			}
		}
	}
	return resourceMatch
}

func interfaceTreeMatch(obj map[string]interface{}, sourceValue string) (bool, string) {
	for key, value := range obj {
		switch v := value.(type) {
		case string:
			if v == sourceValue {
				return true, key
			}
		case []interface{}:
			for _, vv := range v {
				switch vvv := vv.(type) {
				case string:
					if vvv == sourceValue {
						return true, key
					}
				}
			}
		case map[string]interface{}:
			if ok, k := interfaceTreeMatch(v, sourceValue); ok {
				return true, fmt.Sprintf("%s.%s", key, k)
			}
		default:
		}
	}
	return false, ""
}

func getMatchers(typeName, providerName string) []string {
	for _, matcher := range conf.TypeMatchers {
		for _, tName := range matcher.Names {
			for _, pName := range matcher.ProviderMatcher.Names {
				if tName == typeName && pName == providerName {
					return matcher.ProviderMatcher.KeyMatchers
				}
			}
		}
	}
	for _, matcher := range conf.ProviderMatchers {
		for _, pName := range matcher.Names {
			if pName == providerName {
				return matcher.KeyMatchers
			}
		}
	}
	return conf.KeyMatchers
}
