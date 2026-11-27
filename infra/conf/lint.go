package conf

import "github.com/xtls/xray-core/common/errors"

type ConfigureFilePostProcessingStage interface {
	Process(conf *Config) error
}

// postProcessingStage is one stage of the list below, with the name it reports
// when it rejects a configuration.
type postProcessingStage struct {
	name  string
	stage ConfigureFilePostProcessingStage
}

// configureFilePostProcessingStages is the whole list of stages a configuration
// goes through after it is read, one line each. A stage does not register itself
// from an init() function, so this list is the whole picture.
var configureFilePostProcessingStages = []postProcessingStage{
	{name: "FakeDNS", stage: &FakeDNSPostProcessingStage{}},
}

// SetConfigureFilePostProcessingStages replaces the list, for a build that adds
// its own stages.
func SetConfigureFilePostProcessingStages(stages []ConfigureFilePostProcessingStage) {
	list := make([]postProcessingStage, 0, len(stages))
	for _, stage := range stages {
		list = append(list, postProcessingStage{name: "custom", stage: stage})
	}
	configureFilePostProcessingStages = list
}

func PostProcessConfigureFile(conf *Config) error {
	for _, entry := range configureFilePostProcessingStages {
		if err := entry.stage.Process(conf); err != nil {
			return errors.New("Rejected by Postprocessing Stage ", entry.name).Base(err)
		}
	}
	return nil
}
