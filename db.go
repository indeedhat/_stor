package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DBSeperator = "=>"

type DBEntry struct {
	Target  string
	Symlink string
	Comment *string
}

func (e DBEntry) String() string {
	if e.Comment != nil {
		return *e.Comment
	}

	return fmt.Sprintf("%s %s %s\n",
		strconv.Quote(e.Symlink),
		DBSeperator,
		strconv.Quote(e.Target),
	)
}

type DB struct{}

// Store a link pair in the database
func (d DB) Store(ctx Context, target, symlink string) error {
	fh, err := os.OpenFile(dbPath(ctx), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	if _, err := d.Find(ctx, symlink); err == nil {
		return errors.New("Symlink exists in the database")
	}
	if _, err := d.Find(ctx, target); err == nil {
		return errors.New("Target exists in the database")
	}

	symlink = strconv.Quote(symlink)
	target = strconv.Quote(target)

	_, err = fh.WriteString(fmt.Sprintf("%s %s %s\n", symlink, DBSeperator, target))
	return err
}

func (d DB) Find(ctx Context, path string) (*DBEntry, error) {
	entries, err := d.read(ctx)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Comment != nil {
			continue
		}

		if entry.Target == path || entry.Symlink == path {
			return &entry, nil
		}
	}

	return nil, errors.New("path not found")
}

func (d DB) Remove(ctx Context, symlink string) error {
	var (
		buf   bytes.Buffer
		found bool
	)

	entries, err := d.read(ctx)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Comment != nil {
			buf.WriteString(*entry.Comment + "\n")
			continue
		}

		if entry.Symlink == symlink {
			found = true
			continue
		}

		buf.WriteString(fmt.Sprintf("%s %s %s\n",
			strconv.Quote(entry.Target),
			DBSeperator,
			strconv.Quote(entry.Symlink),
		))
	}

	if !found {
		return errors.New("symlink not found")
	}

	return os.WriteFile(dbPath(ctx), buf.Bytes(), 0644)
}

// read the db file into an appropriate data structure
func (DB) read(ctx Context) ([]DBEntry, error) {
	data, err := os.ReadFile(dbPath(ctx))
	if err != nil {
		return nil, errors.New("could not read _stor db")
	}

	var (
		i       int
		db      []DBEntry
		scanner = bufio.NewScanner(bytes.NewReader(data))
	)

	for scanner.Scan() {
		i++
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			db = append(db, DBEntry{Comment: &line})
			continue
		}

		parts := strings.Split(line, DBSeperator)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line in db: %d", i)
		}

		db = append(db, DBEntry{
			Target:  readDbPart(parts[0]),
			Symlink: readDbPart(parts[1]),
		})
	}

	return db, nil
}

func dbPath(ctx Context, altPwd ...string) string {
	if len(altPwd) == 0 {
		altPwd = append(altPwd, ctx.Pwd)
	}
	return filepath.Join(altPwd[0], ".stor")
}

func readDbPart(in string) string {
	out, _ := strconv.Unquote(strings.Trim(in, " \r\n\t"))
	return out
}
