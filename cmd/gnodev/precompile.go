package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno"
	"github.com/gnolang/gno/pkgs/command"
	"github.com/gnolang/gno/pkgs/errors"
)

type precompileOptions struct {
	Verbose     bool   `flag:"verbose" help:"verbose"`
	SkipFmt     bool   `flag:"skip-fmt" help:"do not check syntax of generated .go files"`
	GoBinary    string `flag:"go-binary" help:"go binary to use for building"`
	GofmtBinary string `flag:"go-binary" help:"gofmt binary to use for syntax checking"`
}

var DefaultPrecompileOptions = precompileOptions{
	Verbose:     false,
	SkipFmt:     false,
	GoBinary:    "go",
	GofmtBinary: "gofmt",
}

func precompileApp(cmd *command.Command, args []string, iopts interface{}) error {
	opts := iopts.(precompileOptions)
	if len(args) < 1 {
		cmd.ErrPrintfln("Usage: precompile [precompile flags] [packages]")
		return errors.New("invalid args")
	}

	errCount := 0

	// precompile .gno files.
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("invalid file or package path: %w", err)
		}
		if !info.IsDir() {
			curpath := arg
			err = precompileFile(curpath, opts)
			if err != nil {
				return fmt.Errorf("%s: precompile: %w", curpath, err)
			}
		} else {
			err = filepath.WalkDir(arg, func(curpath string, f fs.DirEntry, err error) error {
				if err != nil {
					return fmt.Errorf("%s: walk dir: %w", arg, err)
				}

				if !isGnoFile(f) {
					return nil // skip
				}
				err = precompileFile(curpath, opts)
				if err != nil {
					err = fmt.Errorf("%s: precompile: %w", curpath, err)
					cmd.ErrPrintfln("%s", err.Error())
					errCount++
					return nil
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	if errCount > 0 {
		return fmt.Errorf("%d precompile errors", errCount)
	}

	return nil
}

func precompileFile(srcPath string, opts precompileOptions) error {
	shouldCheckFmt := !opts.SkipFmt
	verbose := opts.Verbose
	gofmt := opts.GofmtBinary

	if verbose {
		fmt.Fprintf(os.Stderr, "%s\n", srcPath)
	}

	// parse .gno.
	source, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	// preprocess.
	transformed, err := gno.Precompile(string(source))
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// write .go file.
	targetPath := strings.TrimSuffix(srcPath, ".gno") + ".gno.gen.go"
	err = ioutil.WriteFile(targetPath, []byte(transformed), 0o644)
	if err != nil {
		return fmt.Errorf("write .go file: %w", err)
	}

	// check .go fmt.
	if shouldCheckFmt {
		err = gno.PrecompileVerifyFile(targetPath, gofmt)
		if err != nil {
			return fmt.Errorf("check .go file: %w", err)
		}
	}

	return nil
}
