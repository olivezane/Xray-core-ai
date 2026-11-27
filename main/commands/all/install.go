package all

import (
	"github.com/xtls/xray-core/main/commands/all/api"
	"github.com/xtls/xray-core/main/commands/all/base64"
	"github.com/xtls/xray-core/main/commands/all/convert"
	"github.com/xtls/xray-core/main/commands/all/tls"
	"github.com/xtls/xray-core/main/commands/base"
)

// Install wires the commands of this package and adds them to the root command.
// The Run of each command is assigned here, in one place, instead of from an
// init() function in the file that defines the command: reading this function
// tells you the whole command surface of a build.
func Install() {
	cmdUUID.Run = executeUUID
	cmdX25519.Run = executeX25519
	cmdWG.Run = executeWG
	cmdMLDSA65.Run = executeMLDSA65
	cmdMLKEM768.Run = executeMLKEM768
	cmdVLESSEnc.Run = executeVLESSEnc

	base64.Install()
	tls.Install()

	base.RootCommand.Commands = append(
		base.RootCommand.Commands,
		api.CmdAPI,
		base64.CmdBase64,
		convert.CmdConvert,
		tls.CmdTLS,
		cmdUUID,
		cmdX25519,
		cmdWG,
		cmdMLDSA65,
		cmdMLKEM768,
		cmdVLESSEnc,
	)
}
