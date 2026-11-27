package tls

// Install wires the subcommands of tls. It is called from the list that composes
// the command tree, so nothing here registers itself from an init().
func Install() {
	cmdCert.Run = executeCert
	cmdPing.Run = executePing
	cmdHash.Run = executeHash
	cmdECH.Run = executeECH
}
