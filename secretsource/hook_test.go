package secretsource

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretsHookAddIgnoresEmptySecrets(t *testing.T) {
	t.Parallel()

	hook := &secretsHook{}

	// Add an empty secret - this should not cause global redaction
	hook.add("")

	// Add a normal secret
	hook.add("actualsecret")

	entry := &logrus.Entry{
		Message: "This is a test message with actualsecret in it",
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	require.NoError(t, err)

	// The message should have "actualsecret" redacted but should still be readable
	// It should NOT be character-by-character redacted
	assert.Equal(t, "This is a test message with ***SECRET_REDACTED*** in it", entry.Message)
	assert.NotContains(t, entry.Message, "***SECRET_REDACTED***T***SECRET_REDACTED***h***SECRET_REDACTED***")
}

func TestSecretsHook_OnlyEmptySecret(t *testing.T) {
	t.Parallel()

	hook := &secretsHook{}

	// Add only an empty secret
	hook.add("")

	entry := &logrus.Entry{
		Message: "This is a normal message",
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	require.NoError(t, err)

	// The message should remain unchanged
	assert.Equal(t, "This is a normal message", entry.Message)
}

func TestSecretsHook_NormalOperation(t *testing.T) {
	t.Parallel()

	hook := &secretsHook{}

	hook.add("secret123")
	hook.add("anotherSecret")

	entry := &logrus.Entry{
		Message: "Log with secret123 and anotherSecret",
		Data: logrus.Fields{
			"key1": "value with secret123",
			"key2": "value with anotherSecret",
		},
	}

	err := hook.Fire(entry)
	require.NoError(t, err)

	assert.Equal(t, "Log with ***SECRET_REDACTED*** and ***SECRET_REDACTED***", entry.Message)
	assert.Equal(t, "value with ***SECRET_REDACTED***", entry.Data["key1"])
	assert.Equal(t, "value with ***SECRET_REDACTED***", entry.Data["key2"])
}

func TestSecretsHook_RecursiveReplace(t *testing.T) {
	t.Parallel()

	hook := &secretsHook{}
	hook.add("mysecret")

	entry := &logrus.Entry{
		Message: "Test",
		Data: logrus.Fields{
			"stringValue": "contains mysecret here",
			"intValue":    42,
			"floatValue":  3.14,
		},
	}

	err := hook.Fire(entry)
	require.NoError(t, err)

	assert.Equal(t, "contains ***SECRET_REDACTED*** here", entry.Data["stringValue"])
	assert.Equal(t, 42, entry.Data["intValue"])
	assert.Equal(t, 3.14, entry.Data["floatValue"])
}
