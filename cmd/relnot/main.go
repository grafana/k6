// relnot is a tool for generating release notes from the merged
// in a specific milestone.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TODO: It is mostly an MVP, so refactor after we have consensus and we are sure that it makes sense
//
//nolint:funlen,gocognit
func main() {
	var (
		// TODO: we may consider to input it as an argument
		// e.g.
		// $ relnot -m v0.47.0
		milestone = "v0.47.0"

		// TODO: we may consider to input it as an argument
		// e.g.
		// $ relnot -m v0.47.0 ./release\ notes/unreleased.md
		unreleasedFilePath = "./../../release notes/unreleased.md"
	)

	f, err := openUnreleased(unreleasedFilePath)
	if err != nil {
		log.Fatalf("open Unreleased file failed: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	fmt.Println("Unreleased file descriptor opened")

	pulls, err := fetchPullRequests(milestone)
	if err != nil {
		fmt.Println("Fetch pull requests failed:", err)
		return
	}

	if len(pulls) < 1 {
		fmt.Println("There aren't pull requests to process, the generation will be stopped.")
		return
	}

	fmt.Printf("Pull requests fetched: %d\n\n", len(pulls))

	changesByType := make(map[PullType][]change)
	changesContents := make(map[PullType]string)

	log.Println("Parsing changes from Pull request:")
	for _, pull := range pulls {
		fmt.Printf("#%d ", pull.Number)

		var parsedChange change
		parsedChange, err = parseChange(pull)
		if err != nil {
			fmt.Printf("Parse change (#%d) failed: %v\n", parsedChange.Number, err)
			return
		}

		changesByType[parsedChange.Type] = append(changesByType[parsedChange.Type], parsedChange)
		if parsedChange.Type == Undefined {
			continue
		}

		text := parsedChange.Format()
		changesContents[parsedChange.Type] += text + "\n"
	}

	fmt.Println("\n\nMatching stage report:")
	for typ, changes := range changesByType {
		if typ == Undefined {
			fmt.Printf("Type: %s, Count: %d, Pulls: %v\n", typ, len(changes), mapChangesNumbers(changes))
			continue
		}
		fmt.Printf("Type: %s, Count: %d\n", typ, len(changes))
	}

	//nolint:forbidigo //we need to access the file
	ftemp, err := os.CreateTemp("", "")
	if err != nil {
		fmt.Println("Open the temporary Unreleased joiner failed:", err)
		return
	}
	defer func() {
		//nolint:forbidigo
		_ = os.Remove(ftemp.Name())
		_ = ftemp.Close()
	}()

	joiner := bufio.NewWriter(ftemp)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		text := scanner.Text()
		if len(text) < 1 || text[0] != '<' {
			_, err = joiner.Write([]byte(text + "\n"))
			if err != nil {
				fmt.Printf("Failed to write a line copy: %s\n\tthe line: %s\n", err, text)
				return
			}
			continue
		}

		var typeToAdd PullType
		typeToAdd, err = typeFromPlaceholder(text)
		if err != nil {
			fmt.Printf("Parsing of the placeholder (%q) failed: %s\n", text, err)
			return
		}

		changesToAdd, ok := changesContents[typeToAdd]
		if !ok {
			_, err = joiner.Write([]byte(placeholder(typeToAdd) + "\n"))
			if err != nil {
				fmt.Printf("Failed to write a list of changes for the type %q: %s\n", typeToAdd, err)
				return
			}
			continue
		}

		_, err = joiner.Write([]byte(changesToAdd + placeholder(typeToAdd) + "\n"))
		if err != nil {
			fmt.Println("Failed to write a change item:", err)
			return
		}
	}

	if err = scanner.Err(); err != nil {
		fmt.Println("Reading of the current version of the Unreleased file failed:", err)
		return
	}

	err = joiner.Flush()
	if err != nil {
		fmt.Println("\nWARN: The flush operation for the joiner failed:", err)
	}
	_ = ftemp.Close()

	//nolint:forbidigo
	if err := os.Rename(ftemp.Name(), unreleasedFilePath); err != nil {
		fmt.Println("Moving the new unreleased version failed:", err)
		return
	}

	fmt.Println("\nRelease notes generation completed.")
}

func placeholder(typ PullType) string {
	return fmt.Sprintf("<%s>", typ.String())
}

func typeFromPlaceholder(text string) (PullType, error) {
	text = strings.TrimPrefix(text, "<")
	text = strings.TrimSuffix(text, ">")
	return PullTypeString(text)
}

type pullRequest struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	Author struct {
		Username string `json:"login"`
	} `json:"author"`
}

func parseChange(p pullRequest) (change, error) {
	c := change{Number: p.Number}
	_, changeBody, found := strings.Cut(p.Body, "### Changelog\r\n\r\n")
	if !found {
		return c, nil
	}

	bodyParts := strings.SplitN(changeBody, "\r\n\r\n", 2)
	firstLine := strings.SplitN(bodyParts[0], ":", 2)

	changeType, err := PullTypeString(strings.ToLower(firstLine[0]))
	if err != nil {
		return c, fmt.Errorf("pull request's type parser: %w", err)
	}

	c.Number = p.Number
	c.Type = changeType
	c.Title = strings.Trim(firstLine[1], " ")

	if len(bodyParts) > 1 {
		c.Body = bodyParts[1]
	}

	return c, nil
}

func fetchPullRequests(milestone string) ([]pullRequest, error) {
	// It requires pager to be set to `cat` => $ gh config set pager cat

	//nolint:gosec //TODO: validation of milestone
	cmd := exec.Command("gh", "pr", "list", "--repo", "grafana/k6",
		"-s", "merged",
		"--search", fmt.Sprintf("milestone:%s sort:created-desc", milestone),
		"--json", "number,body,author",
		"--limit", "1000",
	)

	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("invoke CLI command: %w", err)
	}

	var pulls []pullRequest
	err = json.Unmarshal(stdout, &pulls)
	if err != nil {
		return nil, fmt.Errorf("JSON unmarhsal: %w", err)
	}

	return pulls, err
}

func mapChangesNumbers(changes []change) []int {
	nums := make([]int, 0, len(changes))
	for _, p := range changes {
		nums = append(nums, p.Number)
	}
	return nums
}

//nolint:forbidigo
func openUnreleased(path string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Clean(path))
	if os.IsNotExist(err) {
		return io.NopCloser(bytes.NewBufferString(template)), nil
	}
	if err != nil {
		return nil, err
	}
	return f, err
}

//nolint:gochecknoglobals
var template = `## Breaking changes

<epic-breaking>

<breaking>

## New features

<epic-feature>

<feature>

### UX improvements and enhancements

<ux>

## Bug fixes

<bug>

## Maintenance and internal improvements

<internal>
`
