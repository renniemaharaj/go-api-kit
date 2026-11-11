package environment

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Read loads literal dotenv values without changing the process environment.
// Missing files are optional; variable expansion and shell commands are not run.
func Read(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}

	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		text = strings.TrimPrefix(text, "export ")
		key, value, found := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		if !found || !validKey(key) {
			return nil, fmt.Errorf("%s:%d: expected an environment assignment", filename, line)
		}

		value, err := parseValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %s: %w", filename, line, key, err)
		}

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}

	return values, nil
}

func validKey(key string) bool {
	if key == "" {
		return false
	}

	for index, char := range key {
		letter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		digit := char >= '0' && char <= '9'
		if !letter && char != '_' && (index == 0 || !digit) {
			return false
		}
	}

	return true
}

func parseValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	quote := value[0]
	if quote != '\'' && quote != '"' {
		for index, char := range value {
			if char == '#' && index > 0 && (value[index-1] == ' ' || value[index-1] == '\t') {
				return strings.TrimSpace(value[:index]), nil
			}
		}

		return value, nil
	}

	escaped := false
	for index := 1; index < len(value); index++ {
		if quote == '"' && !escaped && value[index] == '\\' {
			escaped = true
			continue
		}

		if value[index] == quote && !escaped {
			remainder := strings.TrimSpace(value[index+1:])
			if remainder != "" && !strings.HasPrefix(remainder, "#") {
				return "", errors.New("unexpected text after quoted value")
			}

			if quote == '\'' {
				return value[1:index], nil
			}

			unquoted, err := strconv.Unquote(value[:index+1])
			if err != nil {
				return "", errors.New("invalid quoted value")
			}

			return unquoted, nil
		}

		escaped = false
	}

	return "", errors.New("unterminated quoted value")
}
