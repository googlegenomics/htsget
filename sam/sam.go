// Package sam provides support for parsing SAM files.
package sam

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var tagRe = regexp.MustCompile(`\b(SN|AN):(\S+)\b`)

// GetReferenceID returns the ID of the provided reference name from a SAM file.
func GetReferenceID(r io.Reader, reference string) (int32, error) {
	var current int32

	// @SQ SN:foo LN:5 AN:bar,baz ...
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "@SQ") {
			for _, tag := range tagRe.FindAllStringSubmatch(scanner.Text(), -1) {
				switch tag[1] {
				case "SN":
					if tag[2] == reference {
						return current, nil
					}
				case "AN":
					for _, ref := range strings.Split(tag[2], ",") {
						if reference == ref {
							return current, nil
						}
					}
				}
			}
			current++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("reading header: %v", err)
	}
	return 0, fmt.Errorf("reference %q not found", reference)
}
