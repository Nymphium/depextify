package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nymphium/depextify/depextify"
	"gopkg.in/yaml.v3"
)

// interactive asks which commands of r to ignore and adds them to the ignores of the loaded config file,
// or of ./.depextify.yaml when none was loaded.
func interactive(cfg *CLIConfig, r depextify.ScanResult, stdin io.Reader, stdout io.Writer) error {
	ignored := promptIgnores(bufio.NewScanner(stdin), stdout, r)
	if len(ignored) == 0 {
		_, err := fmt.Fprintln(stdout, "No command was ignored.")
		return err
	}
	path := cfg.configPath
	if path == "" {
		path = ".depextify.yaml"
	}
	if err := addIgnores(path, ignored); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "Added %s to the ignores in %s.\n", strings.Join(ignored, ", "), path)
	return err
}

// promptIgnores asks for each command in r whether to ignore it, and returns the commands to ignore.
// An empty answer keeps the command; the end of the input stops like quit.
func promptIgnores(in *bufio.Scanner, out io.Writer, r depextify.ScanResult) []string {
	var ignored []string
cmds:
	for _, cmd := range r.Commands() {
		uses := 0
		for _, fileOccs := range r {
			uses += len(fileOccs[cmd])
		}
		unit := "uses"
		if uses == 1 {
			unit = "use"
		}
		for {
			_, _ = fmt.Fprintf(out, "%s (%d %s): [k]eep, [i]gnore, [q]uit? ", cmd, uses, unit)
			if !in.Scan() {
				_, _ = fmt.Fprintln(out)
				break cmds
			}
			switch strings.ToLower(strings.TrimSpace(in.Text())) {
			case "", "k", "keep":
				continue cmds
			case "i", "ignore":
				ignored = append(ignored, cmd)
				continue cmds
			case "q", "quit":
				break cmds
			}
		}
	}
	return ignored
}

// addIgnores appends cmds to the ignores list of the YAML config file at path,
// creating the file or the list when missing and keeping the rest of the file.
func addIgnores(path string, cmds []string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var kept []byte
	if len(doc.Content) == 0 {
		// An empty or comment-only file decodes to nothing; keep its text and append a new mapping.
		kept = content
		if len(kept) > 0 && !bytes.HasSuffix(kept, []byte("\n")) {
			kept = append(kept, '\n')
		}
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: the top level is not a mapping", path)
	}

	var list *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "ignores" {
			list = root.Content[i+1]
		}
	}
	switch {
	case list == nil:
		list = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "ignores"}, list)
	case list.Tag == "!!null":
		*list = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	case list.Kind != yaml.SequenceNode:
		return fmt.Errorf("%s: ignores is not a list", path)
	}
	for _, cmd := range cmds {
		// The !!str tag makes the encoder quote names such as "yes" that would otherwise read back as booleans.
		list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: cmd})
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, append(kept, buf.Bytes()...), 0o644)
}
