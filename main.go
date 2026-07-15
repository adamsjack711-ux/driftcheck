// driftcheck — semantic diffing of structured config files across environments.
//
//	driftcheck compare <fileA> <fileB>
//	driftcheck compare-dir <dirA> <dirB>
//
// Exit codes: 0 = no unexpected drift, 1 = drift found, 2 = error.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adamsjack711-ux/driftcheck/internal/diff"
	"github.com/adamsjack711-ux/driftcheck/internal/parse"
	"github.com/adamsjack711-ux/driftcheck/internal/report"
	"github.com/adamsjack711-ux/driftcheck/internal/rules"
)

const version = "0.1.0"

const (
	exitClean = 0
	exitDrift = 1
	exitError = 2
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return exitError
	}
	switch args[0] {
	case "compare":
		return cmdCompare(args[1:], false)
	case "compare-dir":
		return cmdCompare(args[1:], true)
	case "version", "--version", "-v":
		fmt.Println("driftcheck " + version)
		return exitClean
	case "help", "--help", "-h":
		usage(os.Stdout)
		return exitClean
	default:
		fmt.Fprintf(os.Stderr, "driftcheck: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return exitError
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `driftcheck — semantic config drift detection across environments

Usage:
  driftcheck compare <fileA> <fileB>      compare two config files
  driftcheck compare-dir <dirA> <dirB>    compare matching config files in two trees
  driftcheck version

Supported formats: .env, .json, .yaml/.yml, .toml (mixed formats compare fine).

Flags (both commands):
  --json            machine-readable output for CI
  --verbose         also show identical keys and ignored drift
  --show-secrets    print secret values instead of [redacted]
  --config PATH     rules file (default: .driftcheck.yaml in the working dir)
  --no-color        disable ANSI colors (also disabled when not a TTY or NO_COLOR is set)

Exit codes: 0 no unexpected drift · 1 drift found · 2 error (bad args, unreadable
or unparseable file). Parse errors never abort the run; the remaining files are
still compared and the run exits 2.

Rules file (.driftcheck.yaml):
  ignore:                       # key paths expected to differ per environment
    - DATABASE_URL
    - features.*                # '*' matches any characters, dots included
  secret_patterns:              # extra regexes for secret key names
    - internal_cred
  no_default_secrets: false     # disable built-in API_KEY/_TOKEN/_SECRET/PASSWORD patterns
`)
}

type cmdFlags struct {
	json        bool
	verbose     bool
	showSecrets bool
	noColor     bool
	config      string
}

func parseFlags(name string, args []string) (cmdFlags, []string, error) {
	var f cmdFlags
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&f.json, "json", false, "machine-readable output")
	fs.BoolVar(&f.verbose, "verbose", false, "show identical keys and ignored drift")
	fs.BoolVar(&f.showSecrets, "show-secrets", false, "print secret values")
	fs.BoolVar(&f.noColor, "no-color", false, "disable colors")
	fs.StringVar(&f.config, "config", "", "rules file path")
	if err := fs.Parse(args); err != nil {
		return f, nil, err
	}
	return f, fs.Args(), nil
}

func cmdCompare(args []string, dirMode bool) int {
	name := "compare"
	if dirMode {
		name = "compare-dir"
	}
	flags, rest, err := parseFlags(name, args)
	if err != nil {
		return exitError
	}
	if len(rest) != 2 {
		fmt.Fprintf(os.Stderr, "driftcheck %s: expected exactly two arguments, got %d\n", name, len(rest))
		return exitError
	}

	cfgPath, explicit := flags.config, flags.config != ""
	if !explicit {
		cfgPath = rules.DefaultFileName
	}
	cfg, err := rules.Load(cfgPath, explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "driftcheck: %v\n", err)
		return exitError
	}

	var rep *report.Report
	if dirMode {
		rep, err = compareDirs(rest[0], rest[1], cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "driftcheck: %v\n", err)
			return exitError
		}
	} else {
		rep = compareFiles(rest[0], rest[1], cfg)
	}

	opts := report.Options{
		Verbose:     flags.verbose,
		ShowSecrets: flags.showSecrets,
		Color:       useColor(flags),
	}
	if flags.json {
		if err := report.RenderJSON(os.Stdout, rep, opts); err != nil {
			fmt.Fprintf(os.Stderr, "driftcheck: %v\n", err)
			return exitError
		}
	} else {
		report.RenderHuman(os.Stdout, rep, opts)
	}

	switch {
	case len(rep.Errors) > 0:
		return exitError
	case rep.TotalDrift() > 0:
		return exitDrift
	default:
		return exitClean
	}
}

func useColor(f cmdFlags) bool {
	if f.noColor || f.json {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// compareFiles builds a single-pair report. A file that fails to load lands
// in Errors instead of aborting, so the exit code (2) and the message both
// reach CI.
func compareFiles(pathA, pathB string, cfg *rules.Rules) *report.Report {
	rep := &report.Report{}
	src := parse.FileSource{}

	treeA, warnA, errA := src.Load(pathA)
	treeB, warnB, errB := src.Load(pathB)
	if errA != nil {
		rep.Errors = append(rep.Errors, report.FileError{Name: pathA, Err: errA.Error()})
	}
	if errB != nil {
		rep.Errors = append(rep.Errors, report.FileError{Name: pathB, Err: errB.Error()})
	}
	if errA != nil || errB != nil {
		return rep
	}

	rep.Pairs = append(rep.Pairs, report.FilePair{
		NameA:     pathA,
		NameB:     pathB,
		Result:    diff.Compare(treeA, treeB, cfg),
		WarningsA: warnA,
		WarningsB: warnB,
	})
	return rep
}

// skipDirs are directory names never descended into during compare-dir.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true,
	"venv": true, ".venv": true, "vendor": true,
}

// compareDirs walks both trees, pairs config files by relative path
// (treating .yml and .yaml as the same name), and compares each pair.
// Files present on only one side are reported — and counted — as drift.
func compareDirs(dirA, dirB string, cfg *rules.Rules) (*report.Report, error) {
	filesA, err := findConfigFiles(dirA)
	if err != nil {
		return nil, err
	}
	filesB, err := findConfigFiles(dirB)
	if err != nil {
		return nil, err
	}

	rep := &report.Report{DirA: dirA, DirB: dirB}
	src := parse.FileSource{}

	keys := map[string]struct{}{}
	for k := range filesA {
		keys[k] = struct{}{}
	}
	for k := range filesB {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, key := range sorted {
		relA, inA := filesA[key]
		relB, inB := filesB[key]
		switch {
		case inA && !inB:
			rep.OnlyInA = append(rep.OnlyInA, relA)
		case !inA && inB:
			rep.OnlyInB = append(rep.OnlyInB, relB)
		default:
			pathA := filepath.Join(dirA, relA)
			pathB := filepath.Join(dirB, relB)
			treeA, warnA, errA := src.Load(pathA)
			treeB, warnB, errB := src.Load(pathB)
			if errA != nil {
				rep.Errors = append(rep.Errors, report.FileError{Name: pathA, Err: errA.Error()})
			}
			if errB != nil {
				rep.Errors = append(rep.Errors, report.FileError{Name: pathB, Err: errB.Error()})
			}
			if errA != nil || errB != nil {
				continue // report this pair's errors, keep comparing the rest
			}
			rep.Pairs = append(rep.Pairs, report.FilePair{
				NameA:     pathA,
				NameB:     pathB,
				Result:    diff.Compare(treeA, treeB, cfg),
				WarningsA: warnA,
				WarningsB: warnB,
			})
		}
	}
	return rep, nil
}

// findConfigFiles maps a match key (relative path with .yml normalized to
// .yaml) to the actual relative path of every supported config file.
func findConfigFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if parse.DetectFormat(d.Name()) == parse.FormatUnknown {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[matchKey(rel)] = rel
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", root, err)
	}
	return files, nil
}

func matchKey(rel string) string {
	rel = filepath.ToSlash(rel)
	if strings.HasSuffix(rel, ".yml") {
		rel = strings.TrimSuffix(rel, ".yml") + ".yaml"
	}
	return rel
}
