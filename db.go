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

const seperator = "=>"

type DB struct{}

var db = DB{}

// Store a link pair in the database
func (d DB) Store(target, symlink string) error {
	fh, err := os.OpenFile(dbPath(), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	if _, _, err := d.Find(symlink); err == nil {
		return errors.New("Symlink exists in the database")
	}
	if _, _, err := d.Find(target); err == nil {
		return errors.New("Target exists in the database")
	}

	symlink = strconv.Quote(symlink)
	target = strconv.Quote(target)

	_, err = fh.WriteString(fmt.Sprintf("%s %s %s\n", symlink, seperator, target))
	return err
}

func (d DB) Find(path string) (string, string, error) {
	lines, err := d.read()
	if err != nil {
		return "", "", err
	}

	for _, line := range lines {
		if len(line) == 1 {
			continue
		}

		if line[0] == path || line[1] == path {
			return line[0], line[1], nil
		}
	}

	return "", "", errors.New("path not found")
}

func (d DB) Remove(symlink string) error {
	var (
		buf   bytes.Buffer
		found bool
	)

	lines, err := d.read()
	if err != nil {
		return err
	}

	for _, line := range lines {
		if len(line) == 1 {
			buf.WriteString(line[0] + "\n")
			continue
		}

		if line[1] == symlink {
			found = true
			continue
		}

		target := strconv.Quote(line[0])
		symlink := strconv.Quote(line[1])

		buf.WriteString(fmt.Sprintf("%s %s %s\n", target, seperator, symlink))
	}

	if !found {
		return errors.New("symlink not found")
	}

	return os.WriteFile(dbPath(), buf.Bytes(), 0644)
}

// read the db file into an appropriate data structure
func (DB) read() ([][]string, error) {
	data, err := os.ReadFile(dbPath())
	if err != nil {
		return nil, errors.New("could not read .stor db")
	}

	var (
		i       int
		db      [][]string
		scanner = bufio.NewScanner(bytes.NewReader(data))
	)

	for scanner.Scan() {
		i++
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			db = append(db, []string{line})
			continue
		}

		parts := strings.Split(line, seperator)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line in db: %d", i)
		}

		db = append(db, []string{readDbPart(parts[0]), readDbPart(parts[1])})
	}

	return db, nil
}

func dbPath(altPwd ...string) string {
	if len(altPwd) == 0 {
		altPwd = append(altPwd, pwd)
	}
	return filepath.Join(altPwd[0], ".stor")
}

func readDbPart(in string) string {
	out, _ := strconv.Unquote(strings.Trim(in, " \r\n\t"))
	return out
}
