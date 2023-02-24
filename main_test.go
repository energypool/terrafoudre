package main

import (
	tf "github.com/hashicorp/terraform-json"
	"os"
	"reflect"
	"testing"
)

func TestListDestinationStates(t *testing.T) {
	type args struct {
		source string
		dir    string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Right",
			args: args{
				source: "test/states/source.json",
				dir:    "test/states/",
			},
			want: []string{
				"test/states/some_state.json",
			},
		},
		{
			name: "Wrong source",
			args: args{
				source: " ",
				dir:    "test/states/",
			},
			want: []string{
				"test/states/some_state.json",
				"test/states/source.json",
			},
		},
		{
			name: "Wrong dir",
			args: args{
				source: "test/states/source.json",
				dir:    "test/statess/",
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		if got := listDestinationStates(tt.args.source, tt.args.dir); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: listDestinationStates() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestExtractSourceResources(t *testing.T) {
	statefile, err := os.ReadFile("test/states/source.json")
	if err != nil {
		t.Errorf("Can't read state file %s", statefile)
	}
	var state tf.State
	err = state.UnmarshalJSON(statefile)
	if err != nil {
		t.Errorf("Can't read state file %s", statefile)
	}

	type args struct {
		config string
	}
	tests := []struct {
		name string
		args args
		want []SourceResource
	}{
		{
			name: "Extract ID & ARN",
			args: args{
				config: "test/config-full.yaml",
			},
			want: []SourceResource{
				SourceResource{
					Key:          "id",
					Value:        "sg-00000000000000000",
					Provider:     "registry.terraform.io/hashicorp/aws",
					Address:      "module.sg.aws_security_group.sg",
					Destinations: nil,
				},
				SourceResource{
					Key:          "arn",
					Value:        "arn:aws:ec2:eu-west-1:000000000000:security-group/sg-00000000000000000",
					Provider:     "registry.terraform.io/hashicorp/aws",
					Address:      "module.sg.aws_security_group.sg",
					Destinations: nil,
				},
			},
		},
		{
			name: "Extract ID only",
			args: args{
				config: "test/config-short.yaml",
			},
			want: []SourceResource{
				SourceResource{
					Key:          "id",
					Value:        "sg-00000000000000000",
					Provider:     "registry.terraform.io/hashicorp/aws",
					Address:      "module.sg.aws_security_group.sg",
					Destinations: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		conf = Configuration{}
		conf.LoadConfiguration(tt.args.config)
		got := []SourceResource{}
		got = append(got, extractSourceResources(state.Values.RootModule)...)
		for _, child := range state.Values.RootModule.ChildModules {
			got = append(got, extractSourceResources(child)...)
		}

		if !equalsSourceResources(got, tt.want) {
			t.Errorf("%s: extractSourceResources() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAnalyzeDestinationStates(t *testing.T) {
	type args struct {
		config            string
		destinationStates []string
		sourceResources   []SourceResource
	}
	tests := []struct {
		name string
		args args
		want []SourceResource
	}{
		{
			name: "Analyze ID",
			args: args{
				config: "test/config-short.yaml",
				destinationStates: []string{
					"test/states/some_state.json",
				},
				sourceResources: []SourceResource{
					SourceResource{
						Key:          "id",
						Value:        "sg-00000000000000000",
						Provider:     "registry.terraform.io/hashicorp/aws",
						Address:      "module.sg.aws_security_group.sg",
						Destinations: nil,
					},
				},
			},
			want: []SourceResource{
				SourceResource{
					Key:      "id",
					Value:    "sg-00000000000000000",
					Provider: "registry.terraform.io/hashicorp/aws",
					Address:  "module.sg.aws_security_group.sg",
					Destinations: []DestinationResource{
						DestinationResource{
							Provider: "registry.terraform.io/hashicorp/aws",
							Address:  "module.an_awesome_module.data.aws_security_group.sg",
							State:    "test/states/some_state.json",
							Mode:     "data",
							Key:      "id",
						},
					},
				},
			},
		},
		{
			name: "Analyze sub interface",
			args: args{
				config: "test/config-short.yaml",
				destinationStates: []string{
					"test/states/some_state.json",
				},
				sourceResources: []SourceResource{

					SourceResource{
						Key:          "arn",
						Value:        "arn:aws:ec2:eu-west-1:000000000000:security-group/sg-00000000000000000",
						Provider:     "registry.terraform.io/hashicorp/aws",
						Address:      "module.sg.aws_security_group.sg",
						Destinations: nil,
					},
				},
			},
			want: []SourceResource{
				SourceResource{
					Key:      "arn",
					Value:    "arn:aws:ec2:eu-west-1:000000000000:security-group/sg-00000000000000000",
					Provider: "registry.terraform.io/hashicorp/aws",
					Address:  "module.sg.aws_security_group.sg",
					Destinations: []DestinationResource{
						DestinationResource{
							Provider: "registry.terraform.io/hashicorp/aws",
							Address:  "module.an_awesome_module.data.aws_security_group.sg",
							State:    "test/states/some_state.json",
							Mode:     "data",
							Key:      "sub_interface_for_go_tests.arn",
						},
					},
				},
			},
		},
		{
			name: "Analyze sub slice",
			args: args{
				config: "test/config-short.yaml",
				destinationStates: []string{
					"test/states/some_state.json",
				},
				sourceResources: []SourceResource{
					SourceResource{
						Key:          "arn",
						Value:        "arn:aws:test",
						Provider:     "registry.terraform.io/hashicorp/aws",
						Address:      "module.sg.aws_security_group.sg",
						Destinations: nil,
					},
				},
			},
			want: []SourceResource{
				SourceResource{
					Key:      "arn",
					Value:    "arn:aws:test",
					Provider: "registry.terraform.io/hashicorp/aws",
					Address:  "module.sg.aws_security_group.sg",
					Destinations: []DestinationResource{
						DestinationResource{
							Provider: "registry.terraform.io/hashicorp/aws",
							Address:  "module.an_awesome_module.data.aws_security_group.sg",
							State:    "test/states/some_state.json",
							Mode:     "data",
							Key:      "sub_slice_for_go_tests",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		got := analyzeDestinationStates(tt.args.destinationStates, tt.args.sourceResources)

		if !equalsSourceResources(got, tt.want) {
			t.Errorf("%s: analyzeDestinationStates() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func equalsSourceResources(a, b []SourceResource) bool {
	equals := true
	if len(a) == 0 && len(b) == 0 {
		return equals
	}
	if len(a) != len(b) {
		equals = false
	}
	for _, g := range a {
		present := false
		for _, w := range b {
			if w.Key == g.Key &&
				w.Value == g.Value &&
				w.Address == g.Address &&
				w.Provider == g.Provider &&
				equalsDestinationsResources(w.Destinations, g.Destinations) {
				present = true
			}
		}
		if !present {
			equals = false
		}
	}
	return equals
}

func equalsDestinationsResources(a, b []DestinationResource) bool {
	equals := true
	if len(a) != len(b) {
		equals = false
	}
	for _, g := range a {
		present := false
		for _, w := range b {
			if w.Key == g.Key &&
				w.State == g.State &&
				w.Address == g.Address &&
				w.Provider == g.Provider &&
				w.Mode == g.Mode {
				present = true
			}
		}
		if !present {
			equals = false
		}
	}
	return equals
}
