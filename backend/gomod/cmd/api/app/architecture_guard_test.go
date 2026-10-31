package app_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backendRoot walks up from the test's working directory until it finds the
// go.work file, so the guard keeps working wherever it is invoked from.
func backendRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.work not found above the test directory")
		}
		dir = parent
	}
}

// TestArchitectureGuard enforces the dependency rules depguard cannot express
// through package patterns: a service may reach another service only through
// its `<svc>/contract` package, never through its internal packages.
func TestArchitectureGuard(t *testing.T) {
	root := backendRoot(t)
	importRe := regexp.MustCompile(`^\s*(?:[_.]\s*)?"(goshop/services/([^/"]+)/internal/[^"]*)"`)

	var scanned, violations int
	for _, base := range []string{"gomod/services", "gomod/cmd"} {
		err := filepath.Walk(filepath.Join(root, base), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			scanned++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, _ := filepath.Rel(root, path)
			for _, line := range strings.Split(string(content), "\n") {
				m := importRe.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				// Own-package imports are legal; this guard only forbids
				// reaching into a *different* service's internals.
				ownRe := regexp.MustCompile(`^gomod/services/([^/]+)/`)
				own := ownRe.FindStringSubmatch(filepath.ToSlash(rel))
				if own != nil && own[1] == m[2] {
					continue
				}
				violations++
				t.Errorf("%s imports %q; cross-service access must go through %s/contract", rel, m[1], m[2])
			}
			return nil
		})
		require.NoError(t, err)
	}

	assert.NotZero(t, scanned, "no Go files scanned")
	assert.Zero(t, violations)
}

// TestArchitectureGuard_GolangciLinterConfig verifies that .golangci.yml keeps
// the module-boundary depguard rules that enforce the go.work split.
func TestArchitectureGuard_GolangciLinterConfig(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(backendRoot(t), ".golangci.yml"))
	require.NoError(t, err)

	for _, rule := range []string{
		"shared_does_not_depend_on_services",
		"contracts_are_leaf",
		"services_do_not_import_commands",
	} {
		assert.Contains(t, string(content), rule)
	}
}
