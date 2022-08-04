package loader

import "github.com/sirupsen/logrus"

func github(_ logrus.FieldLogger, path string, parts []string) (string, error) {
	username := parts[0]
	repo := parts[1]
	filepath := parts[2]
	return "https://raw.githubusercontent.com/" + username + "/" + repo + "/master/" + filepath, nil
}
