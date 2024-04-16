package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const seperator = " => "

func dbStor(symlink, target string) error {
	fh, err := os.OpenFile(filepath.Join(pwd, ".stor"), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	symlink = strconv.Quote(symlink)
	target = strconv.Quote(target)

	_, err = fh.WriteString(fmt.Sprintf("%s%s%s", symlink, seperator, target))
	return err
}

func dbFind(path string) (string, string, error) {
	r := csv.NewReader()
}

func quoteString(field string) (string, error) {
	var (
		buf bytes.Buffer
		err error
	)

	if err := buf.WriteByte('"'); err != nil {
		return "", err
	}

	for len(field) > 0 {
		i := strings.IndexAny(field, "\"\r\n")
		if i < 0 {
			i = len(field)
		}

		if _, err := buf.WriteString(field[:i]); err != nil {
			return "", err
		}
		field = field[i:]

		if len(field) > 0 {
			switch field[0] {
			case '"':
				_, err = buf.WriteString(`\"`)
			case '\r' | '\n':
				err = buf.WriteByte(field[0])
			}

			field = field[1:]
			if err != nil {
				return "", err
			}
		}
	}

	if err := buf.WriteByte('"'); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func stringNeedsQuotes(field string) bool {
	if field == "" {
		return false
	}

	if field == `\.` {
		return true
	}

	if strings.Contains(field, seperator) {
		return true
	}
	if strings.Contains(field, `"`) {
		return true
	}

	runes := []rune(field)

	return unicode.IsSpace(runes[0]) || unicode.IsSpace(runes[len(runes)-1])
}
