package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/ernestokarim/cb/colors"
	"github.com/ernestokarim/cb/config"
)

// Exec runs a new command and return the output and an error if present.
// It's probably the core of cb as we use external tools for almost anything
// we do.
func Exec(app string, args []string) (string, error) {
	if *config.Verbose {
		log.Printf("%sEXEC%s %s %+v\n", colors.YELLOW, colors.RESET,
			app, args)
	}

	cmd := exec.Command(app, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("exec failed: %s", err)
	}
	return string(output), err
}

// ExecCopyOutput runs a new command and keeps copying the output to stdout
// until it finish. It's used in commands like `cb test` where we need to run
// a permanent app and see the output right as it is produced.
func ExecCopyOutput(app string, args []string) error {
	if *config.Verbose {
		log.Printf("%sEXEC %s %s %+v\n", colors.YELLOW, colors.RESET,
			app, args)
	}

	cmd := exec.Command(app, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot create stdout pipe: %s", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot run the command: %s", err)
	}
	if _, err := io.Copy(os.Stdout, stdout); err != nil {
		return fmt.Errorf("cannot copy the output: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("wait failed: %s", err)
	}

	return nil
}
