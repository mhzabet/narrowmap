package app

import (
	"fmt"
	"strconv"
	"strings"
)

func parseByteSize(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0, fmt.Errorf("size is empty")
	}

	multiplier := int64(1)
	for _, unit := range []struct {
		suffix     string
		multiplier int64
	}{
		{suffix: "GB", multiplier: 1024 * 1024 * 1024},
		{suffix: "MB", multiplier: 1024 * 1024},
		{suffix: "KB", multiplier: 1024},
		{suffix: "B", multiplier: 1},
	} {
		if strings.HasSuffix(value, unit.suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			multiplier = unit.multiplier
			break
		}
	}

	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("use a positive size such as 10MB")
	}
	if number > (1<<63-1)/multiplier {
		return 0, fmt.Errorf("size is too large")
	}
	return number * multiplier, nil
}
