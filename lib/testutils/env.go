/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package testutils

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SetEnv is a helper funcion for setting arbitrary environment variables and
// restoring the old ones at the end, usually by deferring the returned callback
// TODO: remove these hacks when we improve the configuration (hopefully
// completely, see https://github.com/loadimpact/k6/issues/883)... we shouldn't
// have to mess with the global environment at all...
func SetEnv(t *testing.T, newEnv []string) (restoreEnv func()) {
	actuallSetEnv := func(env []string, abortOnSetErr bool) {
		os.Clearenv()
		for _, e := range env {
			val := ""
			pair := strings.SplitN(e, "=", 2)
			if len(pair) > 1 {
				val = pair[1]
			}
			err := os.Setenv(pair[0], val)
			if abortOnSetErr {
				require.NoError(t, err)
			} else if err != nil {
				t.Logf(
					"Received a non-aborting but unexpected error '%s' when setting env.var '%s' to '%s'",
					err, pair[0], val,
				)
			}
		}
	}
	oldEnv := os.Environ()
	actuallSetEnv(newEnv, true)

	return func() {
		actuallSetEnv(oldEnv, false)
	}
}
