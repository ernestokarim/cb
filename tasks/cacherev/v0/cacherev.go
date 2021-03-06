package v0

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ernestokarim/cb/config"
	"github.com/ernestokarim/cb/registry"
	"github.com/ernestokarim/cb/utils"
)

var (
	changes     = map[string]string{}
	allowedExts = map[string]bool{
		".gif":  true,
		".js":   true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".css":  true,
		".otf":  true,
		".eot":  true,
		".svg":  true,
		".ttf":  true,
		".woff": true,
		".ico":  true,
		".txt":  true,
	}
)

func init() {
	registry.NewTask("cacherev", 0, cacherev)
}

func cacherev(c *config.Config, q *registry.Queue) error {
	dirs := c.GetListRequired("cacherev.dirs")
	exclude := c.GetListRequired("cacherev.exclude")
	for _, dir := range dirs {
		dir = filepath.Join("temp", dir)
		if err := filepath.Walk(dir, changeName(exclude)); err != nil {
			return fmt.Errorf("change names walk failed (%s): %s", dir, err)
		}
	}

	rev := c.GetListDefault("cacherev.rev")
	for _, dir := range rev {
		dir = filepath.Join("temp", dir)
		if err := filepath.Walk(dir, changeReferences); err != nil {
			return fmt.Errorf("change references walk failed (%s): %s", dir, err)
		}
	}

	utils.SaveChanges(changes)
	return nil
}

func changeName(excludes []string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("walk failed: %s", err)
		}

		rel, err := filepath.Rel("temp", path)
		if err != nil {
			return fmt.Errorf("cannot rel: %s", err)
		}

		for _, exclude := range excludes {
			if exclude == rel {
				return filepath.SkipDir
			}
		}
		if info.IsDir() {
			return nil
		}
		if !allowedExts[filepath.Ext(path)] {
			return nil
		}

		newname, err := calcNewName(path)
		if err != nil {
			return fmt.Errorf("calc name failed: %s", err)
		}
		newpath := filepath.Join(filepath.Dir(rel), newname)

		changes[rel] = newpath
		if *config.Verbose {
			log.Printf("`%s` converted to `%s`\n", filepath.Base(path), newname)
		}

		abspath := filepath.Join("temp", newpath)
		if err := os.Rename(path, abspath); err != nil {
			return fmt.Errorf("rename failed: %s", err)
		}
		return nil
	}
}

func calcNewName(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open failed: %s", err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read failed: %s", err)
	}

	h := sha1.New()
	if _, err := h.Write(content); err != nil {
		return "", fmt.Errorf("write failed: %s", err)
	}

	enc := fmt.Sprintf("%x", h.Sum(nil))
	return fmt.Sprintf("%s.%s", enc[:8], filepath.Base(path)), nil
}

func changeReferences(path string, info os.FileInfo, err error) error {
	if err != nil {
		return fmt.Errorf("walk failed: %s", err)
	}
	if info.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open failed: %s", err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read failed: %s", err)
	}

	s := string(content)
	for old, change := range changes {
		s = strings.Replace(s, old, change, -1)
	}

	if err := utils.WriteFile(path, s); err != nil {
		return fmt.Errorf("write failed: %s", err)
	}

	return nil
}
