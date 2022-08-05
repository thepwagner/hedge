package debian_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestParseControlFile(t *testing.T) {
	cases := map[string]struct {
		lines    []string
		expected []debian.Paragraph
	}{
		"single line": {
			lines: []string{"Foo: bar"},
			expected: []debian.Paragraph{
				{"Foo": "bar"},
			},
		},
		"multiline key": {
			lines: []string{
				"Foo: bar",
				"SHA256:",
				" 3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
				" 3e9a121d599b56c08bc8f144e4830807c77c29d7114316d6984ba54695d3db7b    57319 contrib/Contents-all.gz",
			},
			expected: []debian.Paragraph{
				{
					"Foo": "bar",
					"SHA256": strings.Join([]string{
						"3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
						"3e9a121d599b56c08bc8f144e4830807c77c29d7114316d6984ba54695d3db7b    57319 contrib/Contents-all.gz",
					}, "\n"),
				},
			},
		},
		"runon key": {
			lines: []string{
				"Tag: game::strategy, interface::graphical, interface::x11, role::program,",
				" uitoolkit::sdl, uitoolkit::wxwidgets, use::gameplaying,",
				" x11::application",
			},
			expected: []debian.Paragraph{
				{
					"Tag": "game::strategy, interface::graphical, interface::x11, role::program, uitoolkit::sdl, uitoolkit::wxwidgets, use::gameplaying, x11::application",
				},
			},
		},
		"multiple paragraphs": {
			lines: []string{
				"Foo: bar",
				"",
				"Foz: baz",
			},
			expected: []debian.Paragraph{
				{"Foo": "bar"},
				{"Foz": "baz"},
			},
		},
	}

	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			graphs, err := debian.ParseControlFile(strings.NewReader(strings.Join(tc.lines, "\n")))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, graphs)
		})
	}
}

func TestWriteControlFile(t *testing.T) {
	cases := map[string]struct {
		paragraphs []debian.Paragraph
		expected   []string
	}{
		"single graph": {
			paragraphs: []debian.Paragraph{
				{"Foo": "bar"},
			},
			expected: []string{
				"Foo: bar",
				"",
			},
		},
		"multiple graphs": {
			paragraphs: []debian.Paragraph{
				{"Foo": "bar"},
				{"Foz": "baz"},
			},
			expected: []string{
				"Foo: bar",
				"",
				"Foz: baz",
				"",
			},
		},
		"multiline keys": {
			paragraphs: []debian.Paragraph{
				{
					"Foo": "bar",
					"SHA256": strings.Join([]string{
						"3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
						"3e9a121d599b56c08bc8f144e4830807c77c29d7114316d6984ba54695d3db7b    57319 contrib/Contents-all.gz",
					}, "\n"),
				},
			},
			expected: []string{
				"Foo: bar",
				"SHA256:",
				" 3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
				" 3e9a121d599b56c08bc8f144e4830807c77c29d7114316d6984ba54695d3db7b    57319 contrib/Contents-all.gz",
				"",
			},
		},
		"field ordering": {
			paragraphs: []debian.Paragraph{
				{
					"Abba":    "Zabba",
					"Package": "test",
					"SHA256":  "3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
					"Zabba":   "Abba",
				},
			},
			expected: []string{
				"Package: test",
				"Abba: Zabba",
				"Zabba: Abba",
				"SHA256:",
				" 3957f28db16e3f28c7b34ae84f1c929c567de6970f3f1b95dac9b498dd80fe63   738242 contrib/Contents-all",
				"",
			},
		},
	}

	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			var buf bytes.Buffer
			err := debian.WriteControlFile(&buf, tc.paragraphs...)
			require.NoError(t, err)

			assert.Equal(t, strings.Join(tc.expected, "\n"), buf.String())

			parsed, err := debian.ParseControlFile(&buf)
			require.NoError(t, err)
			assert.Equal(t, tc.paragraphs, parsed)
		})
	}
}
