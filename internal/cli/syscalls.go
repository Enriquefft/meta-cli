package cli

import "os"

// osExit is the exit function used by CLI commands.
// In production, this calls os.Exit. In tests, this can be overridden.
var osExit = os.Exit
