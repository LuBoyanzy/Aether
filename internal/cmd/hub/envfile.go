package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// loadEnvFileFromDir loads environment variables from a file named "env" under dir.
//
// 1) If the file doesn't exist, it does nothing and returns nil.
// 2) If the file exists but can't be parsed, it returns an error with file/line details.
// 3) It only sets variables that are currently unset or set to an empty value.
func loadEnvFileFromDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("env dir is empty")
	}

	envPath := filepath.Join(dir, "env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read env file %q: %w", envPath, err)
	}

	return loadEnvFromBytes(envPath, b)
}

func loadEnvFromBytes(path string, b []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Allow lines like: export KEY="..."
		if strings.HasPrefix(trimmed, "export ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
		}

		key, val, ok := strings.Cut(trimmed, "=")
		if !ok {
			return fmt.Errorf("%s:%d: invalid line (missing '='): %q", path, lineNo, line)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("%s:%d: invalid line (empty key): %q", path, lineNo, line)
		}
		val = strings.TrimSpace(val)

		// Support "KEY=\"...\"" and "KEY='...'" common formats.
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			unquoted, err := strconv.Unquote(val)
			if err != nil {
				return fmt.Errorf("%s:%d: invalid quoted value for %s: %w", path, lineNo, key, err)
			}
			val = unquoted
		} else if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
			val = val[1 : len(val)-1]
		}

		// Do not override explicitly provided env vars.
		// Treat empty existing values as "unset" to match how the program checks env vars.
		if existing, exists := os.LookupEnv(key); exists && existing != "" {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("%s:%d: failed to set env %s: %w", path, lineNo, key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan env file %q: %w", path, err)
	}
	return nil
}
