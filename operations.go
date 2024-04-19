package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
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
		failedTemplate       = "[FAILED]     %s\n             `%s`\n"
		revertedTemplate     = "[REVERTED]   %s\n             `%s`\n"
		revertFailedTemplate = "[UNREVERTED] %s\n             `%s`\n             Error: %s\n"
		unrevertedTemplate   = "[UNREVERTED] %s\n             `%s`\n"
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
	operations   []Operation
	failIdx      int
	revertReport []error
}

func newPipeline(ops ...Operation) *OpsPipeline {
	return &OpsPipeline{
		operations: ops,
	}
}

func (p *OpsPipeline) Apply() error {
	for i, op := range p.operations {
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

func moveTargetOp(src, dst string) Operation {
	return Operation{
		Name:    "Move target to .stor directory",
		Diagram: fmt.Sprintf("cp %s %s", src, dst),
		Apply: func() error {
			return os.Rename(src, dst)
		},
		Revert: func() error {
			return os.Rename(dst, src)
		},
	}
}

func genSymlinkOp(src, dst string) Operation {
	return Operation{
		Name:    "Symlink files in .stor to original location",
		Diagram: fmt.Sprintf("ln -s %s %s", dst, src),
		Apply: func() error {
			return os.Symlink(src, dst)
		},
		Revert: func() error {
			return os.Remove(dst)
		},
	}
}

func removeSymlinkOp(src, dst string) Operation {
	return Operation{
		Name:    "Remove symlink",
		Diagram: fmt.Sprintf("rm %s", dst),
		Apply: func() error {
			return os.Remove(dst)
		},
		Revert: func() error {
			return os.Symlink(src, dst)
		},
	}
}

func saveToDb(symlink, local string) Operation {
	return Operation{
		Name:    "Save the path pair to the .stor database",
		Diagram: fmt.Sprintf(".stor save %s => %s", symlink, local),
		Apply: func() error {
			return db.Store(symlink, local)
		},
		Revert: func() error {
			return db.Remove(symlink)
		},
	}
}

func removeFromDbOp(symlink, local string) Operation {
	return Operation{
		Name:    "Save the path pair to the .stor database",
		Diagram: fmt.Sprintf(".stor delete %s => %s", symlink, local),
		Apply: func() error {
			return db.Remove(symlink)
		},
		Revert: func() error {
			return db.Store(symlink, local)
		},
	}
}

func sudo() {
	pipe := NewPipeline(
		moveTagetOp(targetPath, destPath),
		genSymlinkOp(destPath, targetPath),
		saveToDbOp(targetPath, destPath),
	)

	if err := pipe.Apply(); err != nil {
		report := pipe.Revert()

		if err := report.Error(); err != nil {
			log.Fatal(err)
		}

		log.Fatal("Track operation failed, all changes were reverted")
	}
}
