package testutil

import (
	"os"
	"path/filepath"

	"recipe-server/config"
)

// FindConfigYAML 从当前工作目录向上查找 config.yaml，返回绝对路径。
func FindConfigYAML() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// LoadConfigIfPresent 若存在 config.yaml 则加载；成功返回 true。
func LoadConfigIfPresent() bool {
	path, ok := FindConfigYAML()
	if !ok {
		return false
	}
	return config.Load(path) == nil
}
