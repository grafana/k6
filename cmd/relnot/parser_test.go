package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestParseChange(t *testing.T) {
	t.Parallel()

	//nolint:forbidigo
	testdata, err := os.Open("./testdata/pulls.json")
	if err != nil {
		t.Fatal(err)
	}

	input, err := io.ReadAll(testdata)
	if err != nil {
		t.Fatal(err)
	}

	var pulls []pullRequest
	err = json.Unmarshal(input, &pulls)
	if err != nil {
		t.Fatal(err)
	}

	changes := make([]change, 0, len(pulls))
	for _, pull := range pulls {
		change, err := parseChange(pull)
		if err != nil {
			t.Fatal(err)
		}
		changes = append(changes, change)
	}

	if len(changes) != 10 {
		t.Errorf("unexpected identified changes")
	}
}
