/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package log

import (
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
)

func parseLevels(level string) ([]logrus.Level, error) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("unknown log level %s", level) // specifically use a custom error
	}
	index := sort.Search(len(logrus.AllLevels), func(i int) bool {
		return logrus.AllLevels[i] > lvl
	})

	return logrus.AllLevels[:index], nil
}
