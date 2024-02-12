// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

export default function (config) {
  function getById(id) {
    return this.filter(
      (/** @type {{ id: string; }} */ element) => element.id == id
    ).at(0);
  }

  Array.prototype["getById"] = getById;

  function durationPanel(suffix) {
    return {
      id: `http_req_duration_${suffix}`,
      title: `HTTP Request Duration ${suffix}`,
      metric: `http_req_duration_trend_${suffix}`,
      format: "duration",
    };
  }

  const overview = config.tabs.getById("overview_snapshot");

  const customPanels = [
    overview.panels.getById("vus"),
    overview.panels.getById("http_reqs"),
    durationPanel("avg"),
    durationPanel("p(90)"),
    durationPanel("p(95)"),
    durationPanel("p(99)"),
  ];

  const durationChart = Object.assign(
    {},
    overview.charts.getById("http_req_duration")
  );

  const customTab = {
    id: "custom",
    title: "Custom",
    event: overview.event,
    panels: customPanels,
    charts: [overview.charts.getById("http_reqs"), durationChart],
    description: "Example of customizing the display of metrics.",
  };

  config.tabs.push(customTab);

  return config;
}
