package lua

// Install wires the lua command. It is called from the list that composes the
// command tree, so nothing here registers itself from an init().
func Install() {
	cmdSign.Run = executeSign
}
