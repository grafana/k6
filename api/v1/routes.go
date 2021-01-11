/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package v1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func NewHandler() http.Handler {
	router := httprouter.New()

	router.GET("/v1/status", HandleGetStatus)
	router.PATCH("/v1/status", HandlePatchStatus)

	router.GET("/v1/metrics", HandleGetMetrics)
	router.GET("/v1/metrics/:id", HandleGetMetric)

	router.GET("/v1/groups", HandleGetGroups)
	router.GET("/v1/groups/:id", HandleGetGroup)

	router.POST("/v1/setup", HandleRunSetup)
	router.PUT("/v1/setup", HandleSetSetupData)
	router.GET("/v1/setup", HandleGetSetupData)

	router.POST("/v1/teardown", HandleRunTeardown)

	router.GET("/v1/monitor", handleGetMonitor)

	return router
}
