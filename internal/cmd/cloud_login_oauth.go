package cmd

import (
	"context"
	"fmt"
	"time"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/cloudapi/oauth"
)

// oauthLoginTimeout bounds how long k6 waits for the user to finish the login
// in their browser before giving up and releasing the callback port.
const oauthLoginTimeout = 5 * time.Minute

// loginWithOAuth authenticates the user in their browser and returns the k6 API
// token and the stack to use, ready for authenticateUserToken.
//
// The browser login yields a short-lived Grafana access token, which k6 uses
// once to read the user's k6 API token and then discards. Only the k6 API token
// is persisted, so this is a more convenient way to obtain the same credential
// `k6 cloud login -t` takes by hand.
//
// The stack serves the login page, so it is required; the user is prompted for
// it when stackInput is empty.
func loginWithOAuth(gs *state.GlobalState, stackInput string) (string, string, error) {
	if stackInput == "" {
		var err error
		if stackInput, err = promptStack(gs); err != nil {
			return "", "", err
		}
	}
	stack := normalizeStackURL(stackInput)

	ctx, cancel := context.WithTimeout(gs.Ctx, oauthLoginTimeout)
	defer cancel()

	flow := &oauth.Flow{StackURL: stack, Out: gs.Stdout}
	result, err := flow.Run(ctx)
	if err != nil {
		return "", "", fmt.Errorf("browser login failed: %w", err)
	}

	if result.Email != "" {
		printToStdout(gs, fmt.Sprintf("\nAuthenticated as %s\n", result.Email))
	}

	// The browser reports which stack it authenticated against. It should be
	// the one k6 sent the user to, but the token belongs to whichever it names,
	// so that one wins.
	if result.StackURL != "" && result.StackURL != stack {
		gs.Logger.Warnf("Logged in to %s rather than the requested %s", result.StackURL, stack)
		stack = result.StackURL
	}

	// The stack is tried as a fallback in case it serves the k6 app plugin's
	// resource API to the access token directly, without the proxy.
	token, host, err := oauth.FetchK6Token(ctx, []string{result.APIBase(), stack}, result.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("could not read your k6 API token: %w", err)
	}
	gs.Logger.Debugf("Read the k6 API token via %s", host)

	return token, stack, nil
}
