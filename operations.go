package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

type Operation struct {
	Name    string
	Diagram string
	Apply   func() error
	Revert  func() error
}

type ReportErr OpsPipeline

func (e *ReportErr) Error() string {
	const (
		failedTemplate       = "\n[FAILED]     %s\n             `%s`\n"
		revertedTemplate     = "\n[REVERTED]   %s\n             `%s`\n"
		revertFailedTemplate = "\n[UNREVERTED] %s\n             `%s`\n             Error: %s\n"
		unrevertedTemplate   = "\n[UNREVERTED] %s\n             `%s`\n"
	)
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf(failedTemplate,
		e.operations[e.failIdx].Name,
		e.operations[e.failIdx].Diagram,
	))

	for i, ei := e.failIdx-1, 0; i > 0; i-- {
		if ei >= len(e.revertReport) {
			buf.WriteString(fmt.Sprintf(unrevertedTemplate,
				e.operations[e.failIdx].Name,
				e.operations[e.failIdx].Diagram,
			))
		} else if err := e.revertReport[ei]; err == nil {
			buf.WriteString(fmt.Sprintf(revertedTemplate,
				e.operations[e.failIdx].Name,
				e.operations[e.failIdx].Diagram,
			))
		} else {
			buf.WriteString(fmt.Sprintf(revertFailedTemplate,
				e.operations[e.failIdx].Name,
				e.operations[e.failIdx].Diagram,
				err,
			))
		}
	}

	return buf.String()
}

var _ error = (*ReportErr)(nil)

type OpsPipeline struct {
	dryRun bool

	operations   []Operation
	failIdx      int
	revertReport []error
}

func newPipeline(dryRun bool, ops ...Operation) *OpsPipeline {
	return &OpsPipeline{
		dryRun:     dryRun,
		operations: ops,
	}
}

func (p *OpsPipeline) Apply() error {
	for i, op := range p.operations {
		if p.dryRun {
			fmt.Println(op.Diagram)
			continue
		}

		if err := op.Apply(); err != nil {
			p.failIdx = i
			return err
		}
	}

	return nil
}

func (p *OpsPipeline) Revert() error {
	// reverse loop the slice skipping the last (first) entry
	for i := len(p.operations) - 2; i > 0; i-- {
		err := p.operations[i].Revert()
		p.revertReport = append(p.revertReport, err)
		if err != nil {
			e := ReportErr(*p)
			return &e
		}
	}

	return nil
}

func moveTargetOp(entry *DBEntry) Operation {
	return Operation{
		Name:    "Move target to _stor directory",
		Diagram: fmt.Sprintf("cp %s %s", entry.Target, entry.Symlink),
		Apply: func() error {
			return os.Rename(entry.Target, entry.Symlink)
		},
		Revert: func() error {
			return os.Rename(entry.Target, entry.Symlink)
		},
	}
}

func genSymlinkOp(entry *DBEntry) Operation {
	return Operation{
		Name:    "Symlink files in _stor to original location",
		Diagram: fmt.Sprintf("ln -s %s %s", entry.Target, entry.Symlink),
		Apply: func() error {
			// NB: this seems backwards but its not
			return os.Symlink(entry.Symlink, entry.Target)
		},
		Revert: func() error {
			return os.Remove(entry.Symlink)
		},
	}
}

func removeSymlinkOp(entry *DBEntry) Operation {
	return Operation{
		Name:    "Remove symlink",
		Diagram: fmt.Sprintf("rm %s", entry.Symlink),
		Apply: func() error {
			return os.Remove(entry.Symlink)
		},
		Revert: func() error {
			return os.Symlink(entry.Target, entry.Symlink)
		},
	}
}

func saveToDb(ctx Context, symlink, local string) Operation {
	return Operation{
		Name:    "Save the path pair to the _stor database",
		Diagram: fmt.Sprintf("_stor save %s => %s", symlink, local),
		Apply: func() error {
			return ctx.DB.Store(ctx, symlink, local)
		},
		Revert: func() error {
			return ctx.DB.Remove(ctx, symlink)
		},
	}
}

func removeFromDbOp(ctx Context, entry *DBEntry) Operation {
	return Operation{
		Name:    "Save the path pair to the _stor database",
		Diagram: fmt.Sprintf("_stor delete %s => %s", entry.Symlink, entry.Target),
		Apply: func() error {
			return ctx.DB.Remove(ctx, entry.Symlink)
		},
		Revert: func() error {
			return ctx.DB.Store(ctx, entry.Symlink, entry.Target)
		},
	}
}

func preApplyScanOp(ctx Context, entries []DBEntry) Operation {
	diagram := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Comment != nil {
			continue
		}

		diagram = append(diagram, fmt.Sprintf("( -f %s && ! -f %s ) \n        || ( $(readlink -f %s) -eq %s/%s )",
			entry.Target,
			entry.Symlink,
			entry.Symlink,
			ctx.Pwd,
			entry.Target,
		))
	}

	return Operation{
		Name:    "Scan the local environment to ensure the _store can be applied",
		Diagram: fmt.Sprintf("[[\n    %s\n]]", strings.Join(diagram, "\n    && ")),
		Apply: func() error {
			var errs []string

			for _, entry := range entries {
				if entry.Comment != nil {
					continue
				}

				if dst, err := os.Readlink(entry.Symlink); err == nil && dst == entry.Target {
					continue
				}

				if stat, err := os.Stat(entry.Target); errors.Is(err, fs.ErrNotExist) {
					errs = append(errs,
						entry.String(),
						fmt.Sprintf("target %s does not exist\n",
							strconv.Quote(entry.Target),
						),
					)
					continue
				} else if stat.Mode()&fs.ModeSymlink == fs.ModeSymlink {
					errs = append(errs,
						entry.String(),
						fmt.Sprintf("target %s is a symlink\n",
							strconv.Quote(entry.Target),
						),
					)
					continue
				}

				if _, err := os.Stat(entry.Symlink); err == nil {
					errs = append(errs,
						entry.String(),
						fmt.Sprintf("destination %s already exists\n",
							strconv.Quote(entry.Symlink),
						),
					)
					continue
				}
			}

			if len(errs) != 0 {
				return fmt.Errorf("Could not apply _stor:\n\n%s", strings.Join(errs, "\n"))
			}

			return nil
		},
		Revert: func() error {
			return nil
		},
	}
}

func applyStorOp(ctx Context, entries []DBEntry) Operation {
	var completed []int
	diagram := make([]string, 0, len(entries))
	for _, entry := range entries {
		diagram = append(diagram, fmt.Sprintf("ln -s %s/%s %s", ctx.Pwd, entry.Target, entry.Symlink))
	}

	return Operation{
		Name:    "Apply missing _stor entries to your environment",
		Diagram: strings.Join(diagram, "\n"),
		Apply: func() error {
			for i, entry := range entries {
				if entry.Comment != nil {
					continue
				}

				if err := os.Symlink(ctx.Pwd+"/"+entry.Target, entry.Symlink); err != nil {
					return err
				}

				completed = append(completed, i)
			}

			return nil
		},
		Revert: func() error {
			var errs []any

			for i, entry := range entries {
				if entry.Comment != nil || slices.Index(completed, i) == -1 {
					continue
				}

				if err := os.Remove(entry.Symlink); err != nil {
					errs = append(errs, err)
				}
			}

			if len(errs) != 0 {
				return fmt.Errorf(strings.Repeat("%w\n", len(errs)), errs...)
			}

			return nil
		},
	}
}
