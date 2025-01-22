(function () {
    var jslib = {};
    (function (module, exports) {
        /*JSLIB_SUMMARY_CODE*/;
    })({ exports: jslib }, jslib);

    var forEach = function (obj, callback) {
        for (var key in obj) {
            if (obj.hasOwnProperty(key)) {
                if (callback(key, obj[key])) {
                    break;
                }
            }
        }
    }

    var transformGroup = function (group) {
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

    var oldJSONSummary = function (data) {
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

    return function (summaryCallbackResult, jsonSummaryPath, data) {
        var result = summaryCallbackResult;
        if (!result) {
            var enableColors = (!data.options.noColor && data.state.isStdOutTTY);
            result = {
                'stdout': '\n' + jslib.textSummary(data, {indent: ' ', enableColors: enableColors}) + '\n\n',
            };
        }

        // TODO: ensure we're returning a map of strings or null/undefined...
        // and if not, log an error and generate the default summary?

        if (jsonSummaryPath != '') {
            result[jsonSummaryPath] = oldJSONSummary(data);
        }

        return result;
    };
})();