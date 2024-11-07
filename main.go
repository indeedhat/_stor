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

var ()

var (
	ErrNotStoreRepo     = errors.New("Not inside an _stor repo")
	ErrAlreadyStoreRepo = errors.New("Current directory is already a _stor repo")
	ErrNoNestedRepos    = errors.New("Cannot create a _stor repo inside another _stor repo")
	ErrInitFailed       = errors.New(
		"Failed to setup a new _stor repo\nPerhaps you don't have write permissions for the current directiory",
	)
	ErrCantTrackSymlink = errors.New("Cannot track a symlink in _stor")
)

func main() {
	ctx := Context{
		DB: DB{},
	}

	var err error
	if ctx.Pwd, err = os.Getwd(); err != nil {
		log.Fatal(err)
	}

	root := rootCmd(ctx)
	root.AddCommand(
		initCmd(ctx),
		trackCmd(ctx),
		releaseCmd(ctx),
		manCmd(ctx, root),
		applyCmd(ctx),
	)

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

// rootCmd is the entrypoint to _stor
func rootCmd(_ Context) *cobra.Command {
	return &cobra.Command{
		Use:   "_stor",
		Short: "Manage your dot files.",
		Long: `_stor provides a simple interface for creating, tracking and applying symlinks on your system to a common directory.
It is designed to allow you to track config files in a git repo, however that's not its only use.`,
		Args:    cobra.NoArgs,
		Version: "0.0.1",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}

// initCmd marks the current directory as a _stor repo
func initCmd(ctx Context) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new _stor repo here.",
		Long:  "Creates a new .stor database in the current directory allowing you to use it as an _stor repository.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isStorRepo(ctx) {
				return ErrAlreadyStoreRepo
			}

			if _, err := storRoot(ctx); err != nil && !errors.Is(err, ErrNotStoreRepo) {
				return ErrNoNestedRepos
			}

			if err := os.WriteFile(filepath.Join(ctx.Pwd, ".stor"), nil, 0644); err != nil {
				return ErrInitFailed
			}

			return nil
		},
	}
}

// trackCmd... tracks a directory in the _stor repo
func trackCmd(ctx Context) *cobra.Command {
	cmd := &cobra.Command{
		Use: "track PATH [STOR_DIR_PATH]",
		Long: `Tracks a target directory in the _stor database.
The original file will be moved to the _store directory and replaced with a symbolic link.`,
		Short: "Track a new path in the _stor repo.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isStorRepo(ctx) {
				return ErrNotStoreRepo
			}

			var dst string

			tgt, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}

			if len(args) == 2 {
				dst = filepath.Join(ctx.Pwd, args[1])
			} else {
				dst = filepath.Join(ctx.Pwd, filepath.Base(tgt))
			}

			parentPath, isSymlink := hasSymlinkParent(ctx, tgt)
			if isSymlink {
				if parentPath == tgt {
					return ErrCantTrackSymlink
				}

				return fmt.Errorf("Cannot target the child of symlink '%s' in _stor", parentPath)
			}

			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("Destination '%s' already exists please provide an alternative destination path", dst)
			}

			entry := DBEntry{Target: tgt, Symlink: dst}

			p := newPipeline(ctx.DryRun, moveTargetOp(&entry), genSymlinkOp(&entry), saveToDb(ctx, tgt, strings.TrimPrefix(dst, ctx.Pwd+"/")))
			if err := p.Apply(); err != nil {
				if rerr := p.Revert(); rerr != nil {
					return fmt.Errorf("%s: %w", err, rerr)
				}

				return fmt.Errorf("%s: Track operation failed, all changes were reverted", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&ctx.DryRun, "dry-run", "d", false, "Print commands rather than running them")

	return cmd
}

// releaseCmd the target pair from the _stor repo reverting changes back to system stock
func releaseCmd(ctx Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release [STOR_DIR_PATH]",
		Short: "Stop tracking a path in the _stor repo.",
		Long: `Removes a path from the _store repository.
The symbolic link will be removed and the target file(s) will be moved back to its original location.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isStorRepo(ctx) {
				return ErrNotStoreRepo
			}

			entry, err := ctx.DB.Find(ctx, args[0])
			if err != nil {
				return err
			}

			p := newPipeline(ctx.DryRun, removeSymlinkOp(entry), moveTargetOp(entry), removeFromDbOp(ctx, entry))
			if err := p.Apply(); err != nil {
				if rerr := p.Revert(); rerr != nil {
					return fmt.Errorf("%s: %w", err, rerr)
				}

				return fmt.Errorf("%s: Release operation failed, all changes were reverted", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&ctx.DryRun, "dry-run", "d", false, "Print commands rather than running them")

	return cmd
}

// applyCmd the current _store repo to the current system
//
// This will attempt to setup any missing symlinks based on the .stor file
// If any symlinks cannot be created no changes will be applied to the system

func applyCmd(ctx Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the current _store repo to your system.",
		Long: `Apply any pending changes to the system.
Any paths tracked in the repository will be symlinked to their target path.
Existing links will be left alone.

Before any changes are applied the source/target paths will be scanned to ensure that all operations can be performed.
If any operations can not be performed then the apply will be abandoned and no changes will be applied to the system.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isStorRepo(ctx) {
				return ErrNotStoreRepo
			}

			entries, err := ctx.DB.read(ctx)
			if err != nil {
				return err
			}

			p := newPipeline(ctx.DryRun, preApplyScanOp(ctx, entries), applyStorOp(ctx, entries))
			if err := p.Apply(); err != nil {
				if rerr := p.Revert(); rerr != nil {
					return fmt.Errorf("%s: %w", err, rerr)
				}

				return fmt.Errorf("%s: Release operation failed, all changes were reverted", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&ctx.DryRun, "dry-run", "d", false, "Print commands rather than running them")

	return cmd
}

// manCmd auto generates man files for the _stor utility
func manCmd(_ Context, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manPage, err := mcobra.NewManPage(1, root)
			if err != nil {
				return err
			}

			fmt.Println(manPage.Build(roff.NewDocument()))
			return nil
		},
	}
}

// storRoot finds the closest parent that is marked as a _stor repo
func storRoot(ctx Context) (string, error) {
	dir := ctx.Pwd

	for len(dir) > 1 {
		stat, err := os.Stat(dbPath(ctx, dir))
		if err == nil && !stat.IsDir() {
			return dir, nil
		}

		dir = filepath.Dir(dir)
	}

	return "", ErrNotStoreRepo
}

// isStorRepo checks if the current working directory is a _stor repo
func isStorRepo(ctx Context) bool {
	path, err := storRoot(ctx)
	if err != nil {
		return false
	}

	return path == ctx.Pwd
}

// hasSymlinkParent walks up the directory tree from the provided path and checks if any in turn
// are a symlink
func hasSymlinkParent(ctx Context, path string) (string, bool) {
	for len(path) > 1 {
		stat, err := os.Lstat(dbPath(ctx, path))
		if err == nil && stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return path, true
		}

		path = filepath.Dir(path)
	}

	return "", false
}
