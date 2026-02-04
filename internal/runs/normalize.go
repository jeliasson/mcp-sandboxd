package runs

import "strings"

func normalizeRunArgs(args *RunSandboxArgs) error {
	if args == nil {
		return nil
	}

	for i := range args.Commands {
		c := &args.Commands[i]
		c.Shell.Value = strings.TrimSpace(c.Shell.Value)
		if c.Shell.String() != "" && len(c.Argv) > 0 {
			// Compatibility: some clients send `shell` as the shell executable
			// and `argv` as the command words.
			// Example observed: shell="/bin/bash" argv=["ls"].
			c.Shell.Value = strings.Join(c.Argv, " ")
			c.Argv = nil
		}
	}
	return nil
}
