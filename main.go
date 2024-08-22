package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
)

var (
	pwd string

	dryRun bool

	rootCmd = &cobra.Command{
		Use:     "_stor",
		Short:   "Manage your dot files",
		Args:    cobra.NoArgs,
		Version: "0.0.1",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize a new _stor repo here",
		Args:  cobra.NoArgs,
		RunE:  initStor,
	}
	trackCmd = &cobra.Command{
		Use:   "track PATH [STOR_DIR_PATH]",
		Short: "Manage your dot files",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  track,
	}
	releaseCmd = &cobra.Command{
		Use:   "release [STOR_DIR_PATH]",
		Short: "Manage your dot files",
		Args:  cobra.ExactArgs(1),
		RunE:  release,
	}
	manCmd = &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manPage, err := mcobra.NewManPage(1, rootCmd) //.
			if err != nil {
				return err
			}

			fmt.Println(manPage.Build(roff.NewDocument()))
			return nil
		},
	}
)

var (
	ErrNotStoreRepo = errors.New("Not inside a _stor repo")
)

func main() {
	var err error
	pwd, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(
		initCmd,
		trackCmd,
		releaseCmd,
		manCmd,
	)

	trackCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print commands rather than running them")
	releaseCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print commands rather than running them")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// initStor marks the current directory as a _stor repo
func initStor(cmd *cobra.Command, args []string) error {
	if isStorRepo() {
		return errors.New("Current directory is already a _stor repo")
	}

	if _, err := storRoot(); err != nil && !errors.Is(err, ErrNotStoreRepo) {
		return errors.New("Cannot create a _stor repo inside another _stor repo")
	}

	if err := os.WriteFile(filepath.Join(pwd, ".stor"), nil, 0644); err != nil {
		return errors.New(
			"Failed to setup a new _stor repo\nPerhaps you don't have write permissions for the current directiory",
		)
	}

	return nil
}

// track... tracks a directory in the .stor
func track(cmd *cobra.Command, args []string) error {
	if !isStorRepo() {
		return errors.New("Current directory is not a _stor repo")
	}

	var dst string

	tgt, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	if len(args) == 2 {
		dst = filepath.Join(pwd, args[1])
	} else {
		dst = filepath.Join(pwd, filepath.Base(tgt))
	}

	parentPath, isSymlink := hasSymlinkParent(tgt)
	if isSymlink {
		if parentPath == tgt {
			return errors.New("Cannot track a symlink in _stor")
		}

		return fmt.Errorf("Cannot target the child of symlink '%s' in _stor", parentPath)
	}

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("Destination '%s' already exists please provide an alternative destination path", dst)
	}

	p := newPipeline(
		moveTargetOp(tgt, dst),
		genSymlinkOp(dst, tgt),
		saveToDb(tgt, strings.TrimPrefix(dst, pwd+"/")),
	)
	p.DryRun = dryRun
	if err := p.Apply(); err != nil {
		if rerr := p.Revert(); rerr != nil {
			return fmt.Errorf("%s: %w", err, rerr)
		}

		return fmt.Errorf("%s: Track operation failed, all changes were reverted", err)
	}

	return nil
}

// release the target pair from the _stor repo reverting changes back to system stock
func release(cmd *cobra.Command, args []string) error {
	if !isStorRepo() {
		return errors.New("Current directory is not a _stor repo")
	}

	target, symlink, err := db.Find(args[0])
	if err != nil {
		return err
	}

	p := newPipeline(removeSymlinkOp(target, symlink), moveTargetOp(target, symlink), removeFromDbOp(symlink, target))
	p.DryRun = dryRun
	if err := p.Apply(); err != nil {
		if rerr := p.Revert(); rerr != nil {
			return fmt.Errorf("%s: %w", err, rerr)
		}

		return fmt.Errorf("%s: Release operation failed, all changes were reverted", err)
	}

	return nil
}

// storRoot finds the closest parent that is marked as a _stor repo
func storRoot() (string, error) {
	dir := pwd

	for len(dir) > 1 {
		stat, err := os.Stat(dbPath(dir))
		if err == nil && !stat.IsDir() {
			return dir, nil
		}

		dir = filepath.Dir(dir)
	}

	return "", ErrNotStoreRepo
}

// isStorRepo checks if the current working directory is a _stor repo
func isStorRepo() bool {
	path, err := storRoot()
	if err != nil {
		return false
	}

	return path == pwd
}

// hasSymlinkParent walks up the directory tree from the provided path and checks if any in turn
// are a symlink
func hasSymlinkParent(path string) (string, bool) {
	for len(path) > 1 {
		stat, err := os.Lstat(dbPath(path))
		if err == nil && stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return path, true
		}

		path = filepath.Dir(path)
	}

	return "", false
}
