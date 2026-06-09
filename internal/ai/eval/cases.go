package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func LoadCases(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []Case
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("%s:%d invalid jsonl case: %w", path, lineNo, err)
		}
		if strings.TrimSpace(c.ID) == "" {
			return nil, fmt.Errorf("%s:%d missing id", path, lineNo)
		}
		if strings.TrimSpace(c.Query) == "" {
			return nil, fmt.Errorf("%s:%d missing query", path, lineNo)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}
