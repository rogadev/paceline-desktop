// Package policy holds repository-wide guarantees that are tested like
// behavior. paceline-tray runs in the background with the user's
// privileges and reads Claude Code's OAuth token, so what each package is
// allowed to do is part of its contract.
package policy

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	root   = "../.."
	module = "github.com/rogadev/paceline-tray"
)

// restricted maps a sensitive import to the only package directories allowed
// to use it. An empty list means no shipped code may import it. Widening a
// list is a security decision: say why in the pull request.
var restricted = map[string][]string{
	// Network access belongs to the one source that polls the usage endpoint.
	"net":        {"internal/usage/oauth"},
	"net/http":   {"internal/usage/oauth"},
	"net/url":    {"internal/usage/oauth"},
	"crypto/tls": {"internal/usage/oauth"},
	// Only the macOS credential lookup may run a process (the `security` CLI).
	"os/exec":  {"internal/creds"},
	"net/rpc":  nil,
	"net/smtp": nil,
	"syscall":  nil, // platform calls go through golang.org/x/sys or a vetted library
	"unsafe":   nil,
	"plugin":   nil,
}

// allowedModules lists every third-party module go.mod may require, with the
// reason it earns a place. See docs/design.md for the planned stack.
var allowedModules = map[string]string{}

// goFiles lists Go files under dirs, relative to root with forward slashes.
func goFiles(t *testing.T, includeTests bool, dirs ...string) []string {
	t.Helper()
	var files []string
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
				return err
			}
			if !includeTests && strings.HasSuffix(p, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		t.Fatal("found no source files; is root correct?")
	}
	return files
}

func TestShippedCodeRespectsImportRestrictions(t *testing.T) {
	fset := token.NewFileSet()
	for _, file := range goFiles(t, false, "cmd", "internal") {
		f, err := parser.ParseFile(fset, filepath.Join(root, file), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		dir := path.Dir(file)
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if allowed, ok := restricted[p]; ok && !slices.Contains(allowed, dir) {
				t.Errorf("%s imports %s; only %v may", file, p, allowed)
			}
			if strings.Contains(p, ".") && !strings.HasPrefix(p, module+"/") && !allowedModule(p) {
				t.Errorf("%s imports %s, which is not from an allowed module", file, p)
			}
		}
	}
}

func allowedModule(importPath string) bool {
	for m := range allowedModules {
		if importPath == m || strings.HasPrefix(importPath, m+"/") {
			return true
		}
	}
	return false
}

func TestGoModRequiresOnlyAllowedModules(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(strings.SplitN(line, "//", 2)[0])
		switch {
		case len(f) == 0:
		case f[0] == "replace":
			t.Errorf("go.mod has a replace directive: %q", line)
		case f[0] == "require" && len(f) >= 2 && f[1] == "(":
			inBlock = true
		case inBlock && f[0] == ")":
			inBlock = false
		case f[0] == "require" && len(f) >= 3:
			checkModule(t, f[1])
		case inBlock:
			checkModule(t, f[0])
		}
	}
}

func checkModule(t *testing.T, mod string) {
	t.Helper()
	if _, ok := allowedModules[mod]; !ok {
		t.Errorf("go.mod requires %s, which is not in allowedModules", mod)
	}
}

// TestGoSourceIsASCII keeps invisible and look-alike characters out of the
// code ("Trojan Source"): a bidi override or zero-width space in a source file
// can make code read differently than it compiles. Non-ASCII text belongs in
// string literals as \u escapes, where reviewers can see it.
func TestGoSourceIsASCII(t *testing.T) {
	for _, file := range goFiles(t, true, "cmd", "internal", "tools") {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		line := 1
		for _, b := range data {
			if b == '\n' {
				line++
			}
			if b > 127 {
				t.Errorf("%s:%d: non-ASCII byte 0x%02x; write it as a \\u escape", file, line, b)
				break
			}
		}
	}
}
