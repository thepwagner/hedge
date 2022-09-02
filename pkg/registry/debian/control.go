package debian

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Interact with data in Debian control file format
// Reference: https://www.debian.org/doc/debian-policy/ch-controlfields.html

// Paragraph is a series of data fields.
type Paragraph map[string]string

type toParagraph interface {
	Paragraph() (Paragraph, error)
}

func (p Paragraph) Paragraph() (Paragraph, error) { return p, nil }

var keyValueRE = regexp.MustCompile(`^([^\s:]+):(.*)$`)

// multilineKeys maintain newlines in their values.
var multilineKeys = map[string]struct{}{
	"MD5Sum": {},
	"SHA256": {},
}

// ParseControlFile parses a Debian control file.
func ParseControlFile(in io.Reader) ([]Paragraph, error) {
	var graphs []Paragraph
	var currentKey string
	currentGraph := Paragraph{}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 512*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()

		// An empty line indicates the end of the current paragraph:
		if len(line) == 0 && len(currentGraph) > 0 {
			graphs = append(graphs, currentGraph)
			currentGraph = Paragraph{}
			currentKey = ""
			continue
		}

		// A line that matches "Key: Value" is a new field (Value may be empty)
		if m := keyValueRE.FindStringSubmatch(line); len(m) > 0 {
			key := string(m[1])
			currentKey = key

			val := strings.TrimSpace(string(m[2]))
			currentGraph[key] = val
			continue
		}

		// A line that starts with a space or tab is a continuation of the current Value
		// Some keys maintain newlines while others are treated as WordWrap.
		if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
			continue
		}
		if _, ok := multilineKeys[currentKey]; ok {
			var prefix string
			if len(currentGraph[currentKey]) > 0 {
				prefix = "\n"
			}
			currentGraph[currentKey] += prefix + strings.TrimSpace(string(line))
		} else {
			currentGraph[currentKey] += string(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if len(currentGraph) > 0 {
		graphs = append(graphs, currentGraph)
	}
	return graphs, nil
}

// WriteConfigFile writes a Debian control file.
func WriteControlFile[P toParagraph](out io.Writer, graphs ...P) error {
	for i, toGraph := range graphs {
		graph, err := toGraph.Paragraph()
		if err != nil {
			return fmt.Errorf("converting to paragraph: %w", err)
		}

		// Blank line after every paragraph as a separator (except the first)
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return fmt.Errorf("writing control file: %w", err)
			}
		}

		// Sort the keys alphabetically, with exceptions for leading/trailing fields:
		keys := make([]string, 0, len(graph))
		for k := range graph {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			keyI := keys[i]
			keyJ := keys[j]
			if keyI == "Package" {
				return true
			} else if keyJ == "Package" {
				return false
			}
			if strings.EqualFold(keyI, "SHA256") {
				return false
			} else if strings.EqualFold(keyJ, "SHA256") {
				return true
			}
			if keyI == "MD5Sum" {
				return false
			} else if keyJ == "MD5Sum" {
				return true
			}
			return strings.Compare(keys[i], keys[j]) < 0
		})

		for _, k := range keys {
			v := graph[k]
			if v == "" {
				continue
			}

			if _, ok := multilineKeys[k]; !ok {
				if _, err := fmt.Fprintf(out, "%s: %s\n", k, v); err != nil {
					return fmt.Errorf("writing single-line key: %w", err)
				}
				continue
			}

			if _, err := fmt.Fprintf(out, "%s:\n", k); err != nil {
				return fmt.Errorf("writing multi-line key: %w", err)
			}
			for _, line := range strings.Split(v, "\n") {
				if _, err := fmt.Fprintf(out, " %s\n", line); err != nil {
					return fmt.Errorf("writing multi-line value: %w", err)
				}
			}
		}
	}
	return nil
}
