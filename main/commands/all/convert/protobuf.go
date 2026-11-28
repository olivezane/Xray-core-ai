package convert

import (
	"github.com/xtls/xray-core/main/commands/base"
)

var cmdProtobuf = &base.Command{
	CustomFlags: true,
	UsageLine:   "{{.Exec}} convert pb [-outpbfile file] [-debug] [-type] [json file] [json file] ...",
	Short:       "Protobuf is not a configuration format (removed)",
	Long: `
Protobuf is not a configuration format any more: a configuration is JSON, YAML or
TOML, and protobuf is only the shape a value takes when it crosses the API.

The command stays so that a script calling it is told why it cannot be served,
instead of failing for a reason that looks like a typo.
`,
	Run: rejectProtobufConfig,
}

func rejectProtobufConfig(cmd *base.Command, args []string) {
	base.Fatalf("protobuf is not a configuration format any more: write JSON, TOML or YAML")
}
