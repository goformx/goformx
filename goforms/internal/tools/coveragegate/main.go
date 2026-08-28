// Command coveragegate rejects statement-coverage regressions in the supported schema-first packages.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	profile := flag.String("profile", ".coverage.out", "Go coverage profile to inspect")
	minimum := flag.Float64("minimum", 0, "minimum statement coverage percentage")
	flag.Parse()

	percentage, err := coverage(*profile)
	if removeErr := os.Remove(*profile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
		err = fmt.Errorf("remove coverage profile: %w", removeErr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("critical schema-first statement coverage: %.1f%% (minimum %.1f%%)\n", percentage, *minimum)
	if percentage < *minimum {
		fmt.Fprintf(os.Stderr, "coverage regression: %.1f%% is below %.1f%%\n", percentage, *minimum)
		os.Exit(1)
	}
}

func coverage(path string) (float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open coverage profile: %w", err)
	}
	defer file.Close()

	var total, covered int
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode: ") {
		return 0, errors.New("invalid coverage profile header")
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			return 0, fmt.Errorf("invalid coverage record %q", scanner.Text())
		}
		statements, parseErr := strconv.Atoi(fields[1])
		if parseErr != nil {
			return 0, fmt.Errorf("parse statement count: %w", parseErr)
		}
		count, parseErr := strconv.Atoi(fields[2])
		if parseErr != nil {
			return 0, fmt.Errorf("parse execution count: %w", parseErr)
		}
		total += statements
		if count > 0 {
			covered += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read coverage profile: %w", err)
	}
	if total == 0 {
		return 0, errors.New("coverage profile contains no statements")
	}
	return float64(covered) * 100 / float64(total), nil
}
