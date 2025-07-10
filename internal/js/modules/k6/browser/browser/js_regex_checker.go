package browser

import (
	"context"
	"fmt"
	"strconv"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// injectURLMatcherScript injects a JavaScript regex checker function into the runtime
// for URL pattern matching. This handles all pattern types: exact match, glob patterns,
// and regex patterns using JavaScript's regex engine for consistency.
func injectURLMatcherScript(ctx context.Context, vu moduleVU, targetID string) (common.JSRegexChecker, error) {
	rt := vu.Runtime()

	const script = `
		function __k6BrowserCheckURLPattern(pattern, url) {
			// Empty pattern matches everything
			if (!pattern || pattern === '') {
				return true;
			}
			
			// Regex pattern (wrapped in slashes)
			if (pattern.length > 2 && pattern[0] === '/' && pattern[pattern.length - 1] === '/') {
				const regex = new RegExp(pattern.slice(1, -1));
				return regex.test(url);
			}
			
			// Exact match
			if (pattern.indexOf('*') === -1 && pattern.indexOf('?') === -1 && pattern.indexOf('[') === -1) {
				return url === pattern;
			}
			
			// Glob pattern - convert to regex
			let regexPattern = pattern;
			// Escape regex special chars (except *, ?, and [])
			regexPattern = regexPattern.replace(/[.+^${}()|\\]/g, '\\$&');
			// Preserve character classes
			regexPattern = regexPattern.replace(/\\\[/g, '[').replace(/\\\]/g, ']');
			// Convert glob patterns
			regexPattern = regexPattern.replace(/\*/g, '.*').replace(/\?/g, '.');
			// Anchor the pattern
			regexPattern = '^' + regexPattern + '$';
			
			const regex = new RegExp(regexPattern);
			return regex.test(url);
		}`
	if _, err := rt.RunString(script); err != nil {
		return nil, fmt.Errorf("injecting URL matcher script: %w", err)
	}

	return func(pattern, url string) (bool, error) {
		var (
			result bool
			err    error
		)

		tq := vu.get(ctx, targetID)
		done := make(chan struct{})

		tq.Queue(func() error {
			defer close(done)

			js := fmt.Sprintf(`__k6BrowserCheckURLPattern(%s, %s)`,
				strconv.Quote(pattern), strconv.Quote(url))

			val, jsErr := rt.RunString(js)
			if jsErr != nil {
				err = fmt.Errorf("evaluating pattern: %w", jsErr)
				return nil
			}

			result = val.ToBoolean()
			return nil
		})

		select {
		case <-done:
		case <-ctx.Done():
			err = fmt.Errorf("context cancelled while evaluating URL pattern")
		}

		return result, err
	}, nil
}
