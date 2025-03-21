(function () {
    // TODO: Find a better name, more descriptive more this variable.
    var jslib = {};
    (function (module, exports) {
        /*JSLIB_SUMMARY_CODE*/;
    })({ exports: jslib }, jslib);

    const forEach = function (obj, callback) {
        for (var key in obj) {
            if (obj.hasOwnProperty(key)) {
                if (callback(key, obj[key])) {
                    break;
                }
            }
        }
    }

    const transformGroup = function (group) {
        if (Array.isArray(group.groups)) {
            var newFormatGroups = group.groups;
            group.groups = {};
            for (var i = 0; i < newFormatGroups.length; i++) {
                group.groups[newFormatGroups[i].name] = transformGroup(newFormatGroups[i]);
            }
        }
        if (Array.isArray(group.checks)) {
            var newFormatChecks = group.checks;
            group.checks = {};
            for (var i = 0; i < newFormatChecks.length; i++) {
                group.checks[newFormatChecks[i].name] = newFormatChecks[i];
            }
        }
        return group;
    };

    const oldJSONSummary = function (data) {
        // Quick copy of the data, since it's easiest to modify it in place.
        var results = JSON.parse(JSON.stringify(data));
        delete results.options;
        delete results.state;

        forEach(results.metrics, function (metricName, metric) {
            var oldFormatMetric = metric.values;
            if (metric.thresholds && Object.keys(metric.thresholds).length > 0) {
                var newFormatThresholds = metric.thresholds;
                oldFormatMetric.thresholds = {};
                forEach(newFormatThresholds, function (thresholdName, threshold) {
                    oldFormatMetric.thresholds[thresholdName] = !threshold.ok;
                });
            }
            if (metric.type == 'rate' && oldFormatMetric.hasOwnProperty('rate')) {
                oldFormatMetric.value = oldFormatMetric.rate; // sigh...
                delete oldFormatMetric.rate;
            }
            results.metrics[metricName] = oldFormatMetric;
        });

        results.root_group = transformGroup(results.root_group);

        return JSON.stringify(results, null, 4);
    };

	const jsonSummary = function (data) {
		const summarizeGroup = function (name, data) {
			const group = {
				name: name,
				checks: {},
				metrics: {},
				groups: {},
			}

			if (data?.checks?.ordered_checks && Array.isArray(data.checks.ordered_checks)) {
				for (const check of data.checks.ordered_checks) {
					group.checks[check.name] = check;
				}
			}

			if (data?.checks?.metrics && typeof data.checks.metrics === 'object') {
				for (const metricName in data.checks.metrics) {
					const {values} = data.checks.metrics[metricName];
					group.metrics[metricName] = {...values};
				}
			}

			if (data?.metrics && typeof data.metrics === 'object') {
				for (const metricsGroupName of Object.keys(data.metrics).sort()) {
					const metrics = data.metrics[metricsGroupName];
					for (const metricName of Object.keys(metrics).sort()) {
						const {values} = metrics[metricName];
						group.metrics[metricName] = {...values};
					}
				}
			}

			if (data?.groups && typeof data.groups === 'object') {
				for (const groupName of Object.keys(data.groups).sort()) {
					group.groups[groupName] = summarizeGroup(groupName, data.groups[groupName]);
				}
			}

			return group
		}

		let results = {
			thresholds: {},
			metrics: {},
			root_group: {},
		};

		if (data?.thresholds && typeof data.thresholds === 'object') {
			for (const metricName of Object.keys(data.thresholds).sort()) {
				let thresholds = {};
				for (const {source, ok} of data.thresholds[metricName].thresholds) {
					thresholds[source] = !ok;
				}
				results.thresholds[metricName] = thresholds;
			}
		}

		results.root_group = summarizeGroup('root_group', data.root_group);
		results.metrics = results.root_group.metrics

		results.root_group.groups = {}
		if (data?.groups && typeof data.groups === 'object') {
			for (const scenarioName of Object.keys(data.groups).sort()) {
				results.root_group.groups[scenarioName] =
					summarizeGroup(scenarioName, data.groups[scenarioName]);
			}
		}

		results.root_group.scenarios = {}
		if (data?.scenarios && typeof data.scenarios === 'object') {
			for (const scenarioName of Object.keys(data.scenarios).sort()) {
				results.root_group.scenarios[scenarioName] =
					summarizeGroup(scenarioName, data.scenarios[scenarioName]);
			}
		}

		return JSON.stringify(results, null, 4);
	};

    return function (summaryCallbackResult, jsonSummaryPath, data, options) {
        let result = summaryCallbackResult;
        if (!result) {
            result = {
                'stdout': '\n' + jslib.textSummary(data, options) + '\n\n',
            };
        }

        // TODO: ensure we're returning a map of strings or null/undefined...
        // and if not, log an error and generate the default summary?

        if (jsonSummaryPath != '') {
            result[jsonSummaryPath] = (() => options.isLegacy
		            ? oldJSONSummary(data)
                : jsonSummary(data)
            )();
        }

        return result;
    };
})();