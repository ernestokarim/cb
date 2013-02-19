package config

import (
	"flag"
)

var (
	Verbose = flag.Bool("v", false, "verbose mode")
	AlwaysY = flag.Bool("y", false, "answer yes to all overwrites")
	AlwaysN = flag.Bool("n", false, "answer no to all overwrites")
)