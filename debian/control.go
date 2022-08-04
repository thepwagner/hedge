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

// ParseControlFile parses a Debian control file.
func ParseControlFile(in io.Reader) ([]Paragraph, error) {
	var paragraphs []Paragraph

	currentGraph := Paragraph{}
	var lastKey string
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()

		// Handle paragraph split:
		if len(line) == 0 && len(currentGraph) > 0 {
			paragraphs = append(paragraphs, currentGraph)
			currentGraph = Paragraph{}
			lastKey = ""
			continue
		}

		// Handle "^Key:" matches
		m := debKeyValue.FindStringSubmatch(line)
		if len(m) > 0 {
			key := string(m[1])
			lastKey = key

			val := strings.TrimSpace(string(m[2]))
			currentGraph[key] = val
			continue
		}

		// Handle values that span multiple lines
		if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
			continue
		}
		if _, ok := multilineKeys[lastKey]; ok {
			// Special values where newlines matter:
			var prefix string
			if len(currentGraph[lastKey]) > 0 {
				prefix = "\n"
			}
			currentGraph[lastKey] += prefix + strings.TrimSpace(string(line))
		} else {
			currentGraph[lastKey] += string(line)
		}
	}

	if len(currentGraph) > 0 {
		paragraphs = append(paragraphs, currentGraph)
	}

	return paragraphs, nil
}

// WriteConfigFile writes a Debian control file.
func WriteControlFile(out io.Writer, graphs ...Paragraph) error {
	for i, graph := range graphs {
		if i > 0 {
			fmt.Fprintln(out)
		}

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
			if keyI == "SHA256" {
				return false
			} else if keyJ == "SHA256" {
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
				fmt.Fprintf(out, "%s: %s\n", k, v)
				continue
			}

			fmt.Fprintf(out, "%s:\n", k)
			for _, line := range strings.Split(v, "\n") {
				fmt.Fprintf(out, " %s\n", line)
			}
		}
	}
	return nil
}

var debKeyValue = regexp.MustCompile(`^([^\s:]+):(.*)$`)

var multilineKeys = map[string]struct{}{
	"MD5Sum": {},
	"SHA256": {},
}
