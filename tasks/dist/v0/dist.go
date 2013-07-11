package v0

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ernestokarim/cb/config"
	"github.com/ernestokarim/cb/registry"
	"github.com/ernestokarim/cb/utils"
)

func init() {
	registry.NewTask("dist:prepare", 0, prepareDist)
	registry.NewTask("dist:copy", 0, copyDist)
}

func prepareDist(c *config.Config, q *registry.Queue) error {
	dirs := c.GetListRequired("prepare_dist")
	for _, from := range dirs {
		to := "temp"
		if strings.Contains(from, "->") {
			parts := strings.Split(from, "->")
			from = strings.TrimSpace(parts[0])
			to = filepath.Join("temp", strings.TrimSpace(parts[1]))
		}

		if _, err := os.Stat(from); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat failed: %s", err)
		}

		output, err := utils.Exec("cp", []string{"-r", from, to})
		if err != nil {
			fmt.Println(output)
			return fmt.Errorf("copy error: %s", err)
		}
	}
	return nil
}

func copyDist(c *config.Config, q *registry.Queue) error {
	dirs := c.GetListRequired("dist")

	changes := utils.LoadChanges()
	for i, dir := range dirs {
		if name, ok := changes[dir]; ok {
			dir = name
		}
		dirs[i] = dir
	}

	for _, dir := range dirs {
		origin := filepath.Join("temp", dir)
		dest := filepath.Join("dist", dir)

		info, err := os.Stat(origin)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat failed: %s", err)
		}
		if !info.IsDir() {
			dir := filepath.Dir(dest)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("prepare dir failed (%s): %s", dir, err)
			}
		}

		if *config.Verbose {
			log.Printf("copy `%s`\n", origin)
		}

		output, err := utils.Exec("cp", []string{"-r", origin, dest})
		if err != nil {
			fmt.Println(output)
			return fmt.Errorf("copy error: %s", err)
		}
	}

	return nil
}
