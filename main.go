package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildIDHashLength matches cmd/go's internal hash length (15 bytes).
const buildIDHashLength = 15

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "bitfield: no tool specified\n")
		os.Exit(1)
	}

	tool := os.Args[1]
	args := os.Args[2:]

	// Only intercept the Go compiler. Pass everything else through.
	if !isCompile(tool) {
		passThrough(tool, args)
		return
	}

	// Handle -V=full: mix in our own hash so the build cache
	// invalidates when bitfield's transformation logic changes.
	if len(args) == 1 && args[0] == "-V=full" {
		handleVersion(tool)
		return
	}

	// Process .go files: run Pass1 + Pass2, write to temp dir, replace args.
	newArgs, err := processCompileArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bitfield: %v\n", err)
		os.Exit(1)
	}

	passThrough(tool, newArgs)
}

// handleVersion intercepts "compile -V=full", captures the real compiler's
// version string, and mixes in a hash of the bitfield binary itself.
// This ensures the Go build cache is invalidated when bitfield changes.
//
// Output format (works for both release and devel):
//
//	<original line> +bitfield buildID=_/_/_/<content_hash>
//
// Go's toolID() handles this correctly in both cases:
//   - release: uses strings.TrimSpace(line) — the suffix changes the ID
//   - devel:   uses contentID(f[len(f)-1]) — parses "buildID=_/_/_/<hash>",
//     extracts <hash> after the last "/"
func handleVersion(tool string) {
	cmd := exec.Command(tool, "-V=full")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bitfield: %v\n", err)
		os.Exit(1)
	}

	line := strings.TrimSpace(string(out))
	f := strings.Fields(line)
	if len(f) < 3 || f[1] != "version" {
		// Unknown format, pass through unchanged.
		fmt.Println(line)
		return
	}

	// Determine the original tool ID the same way Go does.
	var toolID []byte
	if f[2] == "devel" {
		// Devel: content ID is base64-encoded after the last "/" in buildID=...
		last := f[len(f)-1]
		sep := strings.LastIndex(last, "/")
		if sep >= 0 {
			decoded, err := base64.RawURLEncoding.DecodeString(last[sep+1:])
			if err == nil {
				toolID = decoded
			}
		}
	}
	if toolID == nil {
		// Release (or devel fallback): use the whole line.
		toolID = []byte(line)
	}

	// Mix the original tool ID with our own binary's content hash.
	contentID := hashBitfieldContent(toolID)

	// Append "+bitfield buildID=_/_/_/<hash>" — a single format that
	// satisfies Go's toolID() parser for both release and devel builds.
	fmt.Printf("%s +bitfield buildID=_/_/_/%s\n", line,
		base64.RawURLEncoding.EncodeToString(contentID[:buildIDHashLength]))
}

// hashBitfieldContent mixes the given tool ID bytes with bitfield's own
// binary content hash, producing a new SHA-256 digest.
func hashBitfieldContent(toolID []byte) [sha256.Size]byte {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bitfield: cannot locate own executable: %v\n", err)
		os.Exit(1)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bitfield: cannot read own executable: %v\n", err)
		os.Exit(1)
	}

	h := sha256.New()
	h.Write(toolID)
	h.Write(data)
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	return sum
}

// isCompile returns true if the tool path points to the Go compiler.
func isCompile(tool string) bool {
	base := filepath.Base(tool)
	// On Windows it might be compile.exe
	return base == "compile" || base == "compile.exe"
}

// passThrough executes the given tool with args, forwarding stdin/stdout/stderr.
func passThrough(tool string, args []string) {
	cmd := exec.Command(tool, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "bitfield: %v\n", err)
		os.Exit(1)
	}
}

// processCompileArgs finds .go files in the compiler arguments, transforms them,
// and returns new arguments with paths replaced.
func processCompileArgs(args []string) ([]string, error) {
	// Find all .go file arguments.
	var goFiles []string
	goFileIndices := map[int]bool{}

	for i, arg := range args {
		if strings.HasSuffix(arg, ".go") && !strings.HasPrefix(arg, "-") {
			goFiles = append(goFiles, arg)
			goFileIndices[i] = true
		}
	}

	if len(goFiles) == 0 {
		return args, nil
	}

	// Parse all Go files in the package.
	fset := token.NewFileSet()
	var files []*ast.File
	var filePaths []string

	for _, path := range goFiles {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		files = append(files, f)
		filePaths = append(filePaths, path)
	}

	// Pass 1: collect bitfield structs, rewrite struct declarations.
	pkg, err := Pass1(fset, files)
	if err != nil {
		return nil, err
	}

	// If no bitfield structs found, pass args through unchanged.
	if len(pkg.Structs) == 0 {
		return args, nil
	}

	// Pass 2: rewrite field accesses.
	if err := Pass2(fset, files, pkg); err != nil {
		return nil, err
	}

	// Write transformed files to temp directory.
	tmpDir, err := os.MkdirTemp("", "bitfield-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	// Map original paths to temp paths.
	pathMap := make(map[string]string)
	for i, f := range files {
		origPath := filePaths[i]
		baseName := filepath.Base(origPath)
		tmpPath := filepath.Join(tmpDir, baseName)

		var buf bytes.Buffer
		if err := format.Node(&buf, fset, f); err != nil {
			return nil, fmt.Errorf("formatting %s: %w", origPath, err)
		}

		if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", tmpPath, err)
		}

		pathMap[origPath] = tmpPath
	}

	// Replace .go file paths in args.
	newArgs := make([]string, len(args))
	for i, arg := range args {
		if goFileIndices[i] {
			if tmpPath, ok := pathMap[arg]; ok {
				newArgs[i] = tmpPath
				continue
			}
		}
		newArgs[i] = arg
	}

	return newArgs, nil
}
