package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
)

var (
	pwd string

	rootCmd = &cobra.Command{
		Use:     ".stor",
		Short:   "Manage your dot files",
		Args:    cobra.NoArgs,
		Version: "0.0.1",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .stor repo here",
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

func main() {
	var err error
	pwd, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(
		trackCmd,
		releaseCmd,
		manCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// track... tracks a directory in the .stor
func track(cmd *cobra.Command, args []string) error {
	var (
		dst string
		tgt = args[0]
	)

	if len(args) == 2 {
		dst = filepath.Join(pwd, args[1])
	} else {
		dst = filepath.Join(pwd, filepath.Base(tgt))
	}

	parentPath, isSymlink := hasSymlinkParent(tgt)
	if isSymlink {
		if parentPath == tgt {
			return errors.New("Cannot track a symlink in .stor")
		}

		return fmt.Errorf("Cannot target the child of symlink '%s' in .stor", parentPath)
	}

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("Destination '%s' already exists please provide an alternative destination path", dst)
	}

	if err := os.Rename(tgt, dst); err != nil {
		return errors.New("Failed to copy target location to .stor repo")
	}

	if err := os.Symlink(dst, tgt); err != nil {
		if err := os.Rename(tgt, dst); err != nil {
			return errors.New("Failed to create symlink to target, move could not be reverted")
		}
		return errors.New("Failed to create symlink to target, move has been reverted")
	}

	return nil
}

func release(cmd *cobra.Command, args []string) error {
	return nil
}

// initStor marks the current directory as a .stor repo
func initStor(cmd *cobra.Command, args []string) error {
	if isStorRepo() {
		return errors.New("Current directory is already a .stor repo")
	}

	if _, err := storRoot(); err != nil {
		return errors.New("Cannot create a .stor repo inside another .stor repo")
	}

	if err := os.WriteFile(filepath.Join(pwd, ".stor"), nil, 0644); err != nil {
		return errors.New(
			"Failed to setup a new .stor repo\nPerhaps you don't have write permissions for the current directiory",
		)
	}

	return nil
}

// storRoot finds the closest parent that is marked as a .stor repo
func storRoot() (string, error) {
	dir := pwd

	for len(dir) > 1 {
		stat, err := os.Stat(filepath.Join(dir, ".stor"))
		if err == nil && !stat.IsDir() {
			return dir, nil
		}

		dir = filepath.Dir(dir)
	}

	return "", errors.New("Not inside a .stor repo")
}

// isStorRepo checks if the current working directory is a .stor repo
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
		stat, err := os.Lstat(filepath.Join(path, ".stor"))
		if err == nil && stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return path, true
		}

		path = filepath.Dir(path)
	}

	return "", false
}
