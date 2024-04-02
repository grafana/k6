package loader

import "github.com/sirupsen/logrus"

func github(logger logrus.FieldLogger, specifier string, parts []string) (string, error) {
	username := parts[0]
	repo := parts[1]
	filepath := parts[2]
	realURL := "https://raw.githubusercontent.com/" + username + "/" + repo + "/master/" + filepath
	logger.Warnf(magicURLsDeprecationWarning, specifier, "github", realURL)
	return realURL, nil
}

const magicURLsDeprecationWarning = "Specifier %q resolved to use a non-conventional %s loader. " +
	"That loader is deprecated and will be removed in v0.53.0. Please use the real URL %q instead."
