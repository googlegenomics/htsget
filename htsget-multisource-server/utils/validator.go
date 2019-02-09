package utils

import (
	"fmt"
)

func ParseFormat(format string) error {
	if format != "" && format != "BAM" {
		return fmt.Errorf("unsupported format %q", format)
	}
	return nil
}
