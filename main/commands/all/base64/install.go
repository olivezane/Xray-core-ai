package base64

// Install wires the subcommands of base64. It is called from the list that
// composes the command tree, so nothing here registers itself from an init().
func Install() {
	cmdEncode.Run = executeEncode
	cmdDecode.Run = executeDecode
}
