package core

// Flags defines the various flags you can call the account server with. These are used in main
// and passed down to the server code to process.
type Flags struct {
	ConfigFile string

	Debug           bool
	Verbose         bool
	DebugAndVerbose bool
}
