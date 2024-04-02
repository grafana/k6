package loader

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

type cdnjsEnvelope struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Version  string `json:"version"`
	Assets   []struct {
		Version string   `json:"version"`
		Files   []string `json:"files"`
	}
}

func cdnjs(logger logrus.FieldLogger, path string, parts []string) (string, error) {
	name := parts[0]
	version := parts[1]
	filename := parts[2]

	data, err := fetch(logger, "https://api.cdnjs.com/libraries/"+name)
	if err != nil {
		return "", err
	}
	var envelope cdnjsEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}

	// CDNJS doesn't actually send 404s, nonexistent libs' data is just *empty*.
	if envelope.Name == "" {
		return "", fmt.Errorf("cdnjs: no such library: %s", name)
	}

	// If no version is specified, use the default/latest one.
	if version == "" {
		version = envelope.Version
	}

	// If no filename is specified, use the default one, but make sure it actually exists in the
	// chosen version (it may have changed name over the years). If not, the first listed file
	// that does exist in that version is a pretty safe guess.
	if filename == "" {
		filename = envelope.Filename

		backupFilename := filename
		filenameExistsInVersion := false
		for _, ver := range envelope.Assets {
			if ver.Version != version {
				continue
			}
			if len(ver.Files) == 0 {
				return "",
					fmt.Errorf("cdnjs: no files for version %s of %s, this is a problem with the library or cdnjs not k6",
						version, path)
			}
			backupFilename = ver.Files[0]
			for _, file := range ver.Files {
				if file == filename {
					filenameExistsInVersion = true
				}
			}
		}
		if !filenameExistsInVersion {
			filename = backupFilename
		}
	}

	return "https://cdnjs.cloudflare.com/ajax/libs/" + name + "/" + version + "/" + filename, nil
}
