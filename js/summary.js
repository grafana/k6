/**
 * @typedef {Object} Threshold
 * @property {string} source - The threshold expression source.
 * @property {boolean} ok - Whether the threshold was satisfied or not.
 */

/**
 * @typedef {Object} Check
 * @property {string} id - The check ID.
 * @property {string} name - The check name.
 * @property {string} path - The check path.
 * @property {number} passes - The number of successful checks.
 * @property {number} fails - The number of failed checks.
 */

/**
 * @typedef {Object} ReportMetric
 * @property {string} name - The metric name.
 * @property {string} type - The type of the metric (e.g., "counter", "gauge", "rate", "trend").
 * @property {string} contains - The type of data contained in the metric (e.g., "time", "data", "default").
 * @property {Record<string, number>} values - Key-value pairs of metric statistics (e.g. min, max, avg).
 * @property {Threshold[]} [thresholds] - Optional array of thresholds associated with this metric.
 */

/**
 * @typedef {Object} ReportThreshold
 * @property {string} source - The threshold expression source.
 * @property {boolean} ok - Whether the threshold was satisfied or not.
 */

/**
 * @typedef {Object} ReportChecksMetrics
 * @property {ReportMetric[]} total - The total metrics.
 * @property {ReportMetric} success - The successful metrics.
 * @property {ReportMetric} fail - The failed metrics.
 */

/**
 * @typedef {Object} MetricThresholds
 * @property {ReportMetric} metric - The metric object.
 * @property {ReportThreshold[]} thresholds - The thresholds for the metric.
 */

/**
 * @typedef {Object} ReportChecks
 * @property {ReportChecksMetrics} metrics - The metrics for checks.
 * @property {Check[]} ordered_checks - The ordered checks.
 */

/**
 * @typedef {Object} DisplayOptions
 * @property {boolean} sortByName - Whether metrics should be sorted by name.
 * @property {boolean} bold - Whether to display section names in bold.
 * @property {string} indent - Indentation string for the output.
 * @property {string} metricsBlockIndent - Additional indentation for metrics blocks.
 */

/**
 * @typedef {Object} ReportData
 * @property {Record<string, ReportMetric>} metrics - Collection of metrics keyed by their names.
 */

/**
 * A simple iteration utility function for objects.
 *
 * @param {Object} obj - the object to iterate over
 * @param {(key: string, value: any) => (boolean|void)} callback - Callback invoked with (key, value)
 */
var forEach = function (obj, callback) {
	for (var key in obj) {
		if (obj.hasOwnProperty(key)) {
			if (callback(key, obj[key])) {
				break
			}
		}
	}
}

/** A palette of ANSI color codes for terminal output. */
var palette = {
	bold: 1,
	faint: 2,
	red: 31,
	green: 32,
	cyan: 36,
	//TODO: add others?
}

var groupPrefix = '█'
var detailsPrefix = '↳'
var succMark = '✓'
var failMark = '✗'
var defaultOptions = {
	indent: ' ',
	enableColors: true,
	summaryTimeUnit: null,
	summaryTrendStats: null,
	sortByName: true,
}

/**
 * Compute the width of a string as displayed in a terminal, excluding ANSI codes, terminal
 * formatting, Unicode ligatures, etc.
 *
 * @param {string} s - The string to measure
 * @returns {number} The display width of the string
 */
function strWidth(s) {
	// TODO: determine if NFC or NFKD are not more appropriate? or just give up? https://hsivonen.fi/string-length/
	var data = s.normalize('NFKC') // This used to be NFKD in Go, but this should be better
	var inEscSeq = false
	var inLongEscSeq = false
	var width = 0
	for (var char of data) {
		if (char.done) {
			break
		}

		// Skip over ANSI escape codes.
		if (char == '\x1b') {
			inEscSeq = true
			continue
		}
		if (inEscSeq && char == '[') {
			inLongEscSeq = true
			continue
		}
		if (inEscSeq && inLongEscSeq && char.charCodeAt(0) >= 0x40 && char.charCodeAt(0) <= 0x7e) {
			inEscSeq = false
			inLongEscSeq = false
			continue
		}
		if (inEscSeq && !inLongEscSeq && char.charCodeAt(0) >= 0x40 && char.charCodeAt(0) <= 0x5f) {
			inEscSeq = false
			continue
		}

		if (!inEscSeq && !inLongEscSeq) {
			width++
		}
	}
	return width
}

/**
 * Summarizes single check result.
 *
 * @param {string} indent
 * @param {{name: string, passes: number, fails: number}} check - The check object with name, passes and fails
 * @param {(text: string, ...colors: number[]) => string} decorate - A function to apply ANSI colors.
 * @returns {string} - A formatted line summarizing the check.
 */
function summarizeCheck(indent, check, decorate) {
	if (check.fails == 0) {
		return decorate(indent + succMark + ' ' + check.name, palette.green)
	}

	var succPercent = Math.floor((100 * check.passes) / (check.passes + check.fails))
	return decorate(
		indent +
		failMark +
		' ' +
		check.name +
		'\n' +
		indent +
		' ' +
		detailsPrefix +
		'  ' +
		succPercent +
		'% — ' +
		succMark +
		' ' +
		check.passes +
		' / ' +
		failMark +
		' ' +
		check.fails,
		palette.red
	)
}

/**
 * Summarizes a group of checks, recursively handling nested groups.
 *
 * @param {string} indent -The indentation for the group.
 * @param {{name: string, checks: Object[], groups: Object[]}} group - Group object with name, checks, and subgroups.
 * @param {(text: string, ...colors: number[]) => string} decorate = Function to decorate text with ANSI colors.
 * @returns {string[]} - An array of formatted lines summarizing the group and its checks
 */
function summarizeGroup(indent, group, decorate) {
	var result = []
	if (group.name != '') {
		result.push(indent + groupPrefix + ' ' + group.name + '\n')
		indent = indent + '  '
	}

	for (var i = 0; i < group.checks.length; i++) {
		result.push(summarizeCheck(indent, group.checks[i], decorate))
	}
	if (group.checks.length > 0) {
		result.push('')
	}
	for (var i = 0; i < group.groups.length; i++) {
		Array.prototype.push.apply(result, summarizeGroup(indent, group.groups[i], decorate))
	}

	return result
}

/**
 * Extracts a display name for a metric, handling sub-metrics (e.g. "metric{sub}" -> "{ sub }").
 *
 * @param {string} name - The metric name.
 * @returns {string} - The display name
 */
function displayNameForMetric(name) {
	var subMetricPos = name.indexOf('{')
	if (subMetricPos >= 0) {
		return '{ ' + name.substring(subMetricPos + 1, name.length - 1) + ' }'
	}
	return name
}

/**
 * Determines the indentation for a metric line based on whether it has submetrics.
 *
 * @param {string} name - The metric name.
 * @returns {string} - Indentation string.
 */
function indentForMetric(name) {
	if (name.indexOf('{') >= 0) {
		return '  '
	}
	return ''
}

/**
 * Converts a number of bytes into a human-readable string with units.
 *
 * @param {number} bytes - The number of bytes.
 * @returns {string} A human-readable string (e.g. "10 kB").
 */
function humanizeBytes(bytes) {
	var units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
	var base = 1000
	if (bytes < 10) {
		return bytes + ' B'
	}

	var e = Math.floor(Math.log(bytes) / Math.log(base))
	var suffix = units[e | 0]
	var val = Math.floor((bytes / Math.pow(base, e)) * 10 + 0.5) / 10
	return val.toFixed(val < 10 ? 1 : 0) + ' ' + suffix
}

var unitMap = {
	s: {unit: 's', coef: 0.001},
	ms: {unit: 'ms', coef: 1},
	us: {unit: 'µs', coef: 1000},
}

/**
 * Converts a number to a fixed decimal string, removing trailing zeros.
 *
 * @param {number} val - The number to convert.
 * @param {number} prec - Decimal precision.
 * @returns {string} A string representation of the number without trailing zeros.
 */
function toFixedNoTrailingZeros(val, prec) {
	// TODO: figure out something better?
	return parseFloat(val.toFixed(prec)).toString()
}

/**
 * Truncates a number to a certain precision without rounding, then removes trailing zeros.
 *
 * @param {number} val - The number to truncate.
 * @param {number} prec - Decimal precision.
 * @returns {string} A truncated, not rounded string representation.
 */
function toFixedNoTrailingZerosTrunc(val, prec) {
	var mult = Math.pow(10, prec)
	return toFixedNoTrailingZeros(Math.trunc(mult * val) / mult, prec)
}

/**
 * Humanizes a duration (in milliseconds) to a human-readable string,
 * choosing appropriate units (ns, µs, ms, s, m, h).
 *
 * @param {number} dur - The duration in milliseconds.
 * @returns {string} Human-readable duration (e.g. "2.5ms", "3s", "1m30s").
 */
function humanizeGenericDuration(dur) {
	if (dur === 0) {
		return '0s'
	}

	if (dur < 0.001) {
		// smaller than a microsecond, print nanoseconds
		return Math.trunc(dur * 1000000) + 'ns'
	}
	if (dur < 1) {
		// smaller than a millisecond, print microseconds
		return toFixedNoTrailingZerosTrunc(dur * 1000, 2) + 'µs'
	}
	if (dur < 1000) {
		// duration is smaller than a second
		return toFixedNoTrailingZerosTrunc(dur, 2) + 'ms'
	}

	var result = toFixedNoTrailingZerosTrunc((dur % 60000) / 1000, dur > 60000 ? 0 : 2) + 's'
	var rem = Math.trunc(dur / 60000)
	if (rem < 1) {
		// less than a minute
		return result
	}
	result = (rem % 60) + 'm' + result
	rem = Math.trunc(rem / 60)
	if (rem < 1) {
		// less than an hour
		return result
	}
	return rem + 'h' + result
}

/**
 * Humanizes a duration according to a specified time unit or uses a generic formatting.
 *
 * @param {number} dur - The duration in milliseconds.
 * @param {string|null} timeUnit - Optional time unit (e.g. "ms", "s").
 * @returns {string} A human-readable duration string.
 */
function humanizeDuration(dur, timeUnit) {
	if (timeUnit !== '' && unitMap.hasOwnProperty(timeUnit)) {
		return (dur * unitMap[timeUnit].coef).toFixed(2) + unitMap[timeUnit].unit
	}

	return humanizeGenericDuration(dur)
}

/**
 * Formats a metric value into a human-readable form, depending on the metric type and content.
 *
 * @param {number} val - The metric value.
 * @param {ReportMetric} metric - The metric object.
 * @param {string|null} timeUnit - The time unit for duration metrics.
 * @returns {string} The humanized metric value.
 */
function humanizeValue(val, metric, timeUnit) {
	if (metric.type == 'rate') {
		// Truncate instead of round when decreasing precision to 2 decimal places
		return (Math.trunc(val * 100 * 100) / 100).toFixed(2) + '%'
	}

	switch (metric.contains) {
		case 'data':
			return humanizeBytes(val)
		case 'time':
			return humanizeDuration(val, timeUnit)
		default:
			return toFixedNoTrailingZeros(val, 6)
	}
}

/**
 * Returns the summary values for non-trend metrics (counter, gauge, rate).
 *
 * @param {ReportMetric} metric - The metric to summarize.
 * @param {string|null} timeUnit - The time unit for durations.
 * @returns {string[]} - An array of summary values.
 */
function nonTrendMetricValueForSum(metric, timeUnit) {
	switch (metric.type) {
		case 'counter':
			return [
				humanizeValue(metric.values.count, metric, timeUnit),
				humanizeValue(metric.values.rate, metric, timeUnit) + '/s',
			]
		case 'gauge':
			return [
				humanizeValue(metric.values.value, metric, timeUnit),
				'min=' + humanizeValue(metric.values.min, metric, timeUnit),
				'max=' + humanizeValue(metric.values.max, metric, timeUnit),
			]
		case 'rate':
			return [
				humanizeValue(metric.values.rate, metric, timeUnit),
				`${metric.values.passes} out of ${metric.values.passes + metric.values.fails}`,
			]
		default:
			return ['[no data]']
	}
}

// FIXME (@oleiade) split this code up for reusability (for instance in the summarizeThreshold function below)
/**
 * Summarizes given metrics into an array of formatted lines.
 *
 * @param {Object} options - Display options merged with defaultOptions.
 * @param {ReportData} data - The data object containing metrics.
 * @param {(text: string, ...colors: number[]) => string} decorate - A decoration function for ANSI colors.
 * @returns {string[]} Array of formatted lines.
 */
function summarizeMetrics(options, data, decorate) {
	var indent = options.indent + ' '
	var result = []

	var names = []
	var nameLenMax = 0

	var nonTrendValues = {}
	var nonTrendValueMaxLen = 0
	var nonTrendExtras = {}
	var nonTrendExtraMaxLens = [0, 0]

	var trendCols = {}
	var numTrendColumns = options.summaryTrendStats.length
	var trendColMaxLens = new Array(numTrendColumns).fill(0)
	forEach(data.metrics, function (name, metric) {
		names.push(name)
		// When calculating widths for metrics, account for the indentation on submetrics.
		var displayName = indentForMetric(name) + displayNameForMetric(name)
		var displayNameWidth = strWidth(displayName)
		if (displayNameWidth > nameLenMax) {
			nameLenMax = displayNameWidth
		}

		if (metric.type == 'trend') {
			var cols = []
			for (var i = 0; i < numTrendColumns; i++) {
				var tc = options.summaryTrendStats[i]
				var value = metric.values[tc]
				if (tc === 'count') {
					value = value.toString()
				} else {
					value = humanizeValue(value, metric, options.summaryTimeUnit)
				}
				var valLen = strWidth(value)
				if (valLen > trendColMaxLens[i]) {
					trendColMaxLens[i] = valLen
				}
				cols[i] = value
			}
			trendCols[name] = cols
			return
		}
		var values = nonTrendMetricValueForSum(metric, options.summaryTimeUnit)
		nonTrendValues[name] = values[0]
		var valueLen = strWidth(values[0])
		if (valueLen > nonTrendValueMaxLen) {
			nonTrendValueMaxLen = valueLen
		}
		nonTrendExtras[name] = values.slice(1)
		for (var i = 1; i < values.length; i++) {
			var extraLen = strWidth(values[i])
			if (extraLen > nonTrendExtraMaxLens[i - 1]) {
				nonTrendExtraMaxLens[i - 1] = extraLen
			}
		}
	})

	// sort all metrics but keep sub metrics grouped with their parent metrics
	if (options.sortByName) {
		names.sort(function (metric1, metric2) {
			var parent1 = metric1.split('{', 1)[0]
			var parent2 = metric2.split('{', 1)[0]
			var result = parent1.localeCompare(parent2)
			if (result !== 0) {
				return result
			}
			var sub1 = metric1.substring(parent1.length)
			var sub2 = metric2.substring(parent2.length)
			return sub1.localeCompare(sub2)
		})
	}

	var getData = function (name) {
		if (trendCols.hasOwnProperty(name)) {
			var cols = trendCols[name]
			var tmpCols = new Array(numTrendColumns)
			for (var i = 0; i < cols.length; i++) {
				tmpCols[i] =
					options.summaryTrendStats[i] +
					'=' +
					decorate(cols[i], palette.cyan) +
					' '.repeat(trendColMaxLens[i] - strWidth(cols[i]))
			}
			return tmpCols.join(' ')
		}

		var value = nonTrendValues[name]
		var fmtData = decorate(value, palette.cyan) + ' '.repeat(nonTrendValueMaxLen - strWidth(value))

		var extras = nonTrendExtras[name]
		if (extras.length == 1) {
			fmtData = fmtData + ' ' + decorate(extras[0], palette.cyan, palette.faint)
		} else if (extras.length > 1) {
			var parts = new Array(extras.length)
			for (var i = 0; i < extras.length; i++) {
				parts[i] =
					decorate(extras[i], palette.cyan, palette.faint) +
					' '.repeat(nonTrendExtraMaxLens[i] - strWidth(extras[i]))
			}
			fmtData = fmtData + ' ' + parts.join(' ')
		}

		return fmtData
	}

	for (var name of names) {
		var metric = data.metrics[name]
		var mark = ' '
		var markColor = function (text) {
			return text
		} // noop

		if (metric.thresholds) {
			mark = succMark
			markColor = function (text) {
				return decorate(text, palette.green)
			}
			forEach(metric.thresholds, function (name, threshold) {
				if (!threshold.ok) {
					mark = failMark
					markColor = function (text) {
						return decorate(text, palette.red)
					}
					return true // break
				}
			})
		}
		var fmtIndent = indentForMetric(name)
		var fmtName = displayNameForMetric(name)
		fmtName =
			fmtName +
			decorate(
				'.'.repeat(nameLenMax - strWidth(fmtName) - strWidth(fmtIndent) + 3) + ':',
				palette.faint
			)

		result.push(indent + fmtIndent + markColor(mark) + ' ' + fmtName + ' ' + getData(name))
	}

	return result
}

/**
 * Summarizes metrics and their thresholds into formatted lines.
 *
 * @param {Object} options - Options merged with defaults.
 * @param {ReportData} data - The data containing metrics.
 * @param {(text: string, ...colors: number[]) => string} decorate - Decoration function.
 * @returns {string[]} - Array of formatted lines including threshold statuses.
 */
function summarizeMetricsWithThresholds(options, data, decorate) {
	var indent = options.indent + ' '
	var result = []

	var names = []
	var nameLenMax = 0

	var nonTrendValues = {}
	var nonTrendValueMaxLen = 0
	var nonTrendExtras = {}
	var nonTrendExtraMaxLens = [0, 0]

	var trendCols = {}
	var numTrendColumns = options.summaryTrendStats.length
	var trendColMaxLens = new Array(numTrendColumns).fill(0)
	forEach(data.metrics, function (name, metric) {
		names.push(name)
		// When calculating widths for metrics, account for the indentation on submetrics.
		var displayNameWidth = strWidth(name)
		if (displayNameWidth > nameLenMax) {
			nameLenMax = displayNameWidth
		}

		if (metric.type == 'trend') {
			var cols = []
			for (var i = 0; i < numTrendColumns; i++) {
				var tc = options.summaryTrendStats[i]
				var value = metric.values[tc]
				if (tc === 'count') {
					value = value.toString()
				} else {
					value = humanizeValue(value, metric, options.summaryTimeUnit)
				}
				var valLen = strWidth(value)
				if (valLen > trendColMaxLens[i]) {
					trendColMaxLens[i] = valLen
				}
				cols[i] = value
			}
			trendCols[name] = cols
			return
		}
		var values = nonTrendMetricValueForSum(metric, options.summaryTimeUnit)
		nonTrendValues[name] = values[0]
		var valueLen = strWidth(values[0])
		if (valueLen > nonTrendValueMaxLen) {
			nonTrendValueMaxLen = valueLen
		}
		nonTrendExtras[name] = values.slice(1)
		for (var i = 1; i < values.length; i++) {
			var extraLen = strWidth(values[i])
			if (extraLen > nonTrendExtraMaxLens[i - 1]) {
				nonTrendExtraMaxLens[i - 1] = extraLen
			}
		}
	})

	// sort all metrics but keep sub metrics grouped with their parent metrics
	if (options.sortByName) {
		names.sort(function (metric1, metric2) {
			var parent1 = metric1.split('{', 1)[0]
			var parent2 = metric2.split('{', 1)[0]
			var result = parent1.localeCompare(parent2)
			if (result !== 0) {
				return result
			}
			var sub1 = metric1.substring(parent1.length)
			var sub2 = metric2.substring(parent2.length)
			return sub1.localeCompare(sub2)
		})
	}

	var getData = function (name) {
		if (trendCols.hasOwnProperty(name)) {
			var cols = trendCols[name]
			var tmpCols = new Array(numTrendColumns)
			for (var i = 0; i < cols.length; i++) {
				tmpCols[i] =
					options.summaryTrendStats[i] +
					'=' +
					decorate(cols[i], palette.cyan) +
					' '.repeat(trendColMaxLens[i] - strWidth(cols[i]))
			}
			return tmpCols.join(' ')
		}

		var value = nonTrendValues[name]
		var fmtData = decorate(value, palette.cyan) + ' '.repeat(nonTrendValueMaxLen - strWidth(value))

		var extras = nonTrendExtras[name]
		if (extras.length == 1) {
			fmtData = fmtData + ' ' + decorate(extras[0], palette.cyan, palette.faint)
		} else if (extras.length > 1) {
			var parts = new Array(extras.length)
			for (var i = 0; i < extras.length; i++) {
				parts[i] =
					decorate(extras[i], palette.cyan, palette.faint) +
					' '.repeat(nonTrendExtraMaxLens[i] - strWidth(extras[i]))
			}
			fmtData = fmtData + ' ' + parts.join(' ')
		}

		return fmtData
	}

	for (var name of names) {
		var metric = data.metrics[name]
		var mark = ' '
		var markColor = function (text) {
			return text
		} // noop

		var fmtName =
			name +
			decorate(
				'.'.repeat(nameLenMax - strWidth(name) + 3) + ':',
				palette.faint
			)

		result.push(indent + markColor(mark) + ' ' + fmtName + ' ' + getData(name))
		if (metric.thresholds) {
			forEach(metric.thresholds, function (name, threshold) {
				const resultIndent = threshold.ok ? '    ' : '  ';
				const thresholdResult = threshold.ok ? decorate('SATISFIED', palette.green) : decorate('UNSATISFIED', palette.red);
				result.push(indent + indent + '  ' + thresholdResult + resultIndent + decorate(`'${threshold.source}'`, palette.faint))
			})
		}
	}

	return result
}

/**
 * Generates a textual summary of test results, including checks, metrics, thresholds, groups, and scenarios.
 *
 * @param {Object} data - The data input for the summary (includes options, metrics, etc.).
 * @param {Object} options - Additional options that override defaults.
 * @param {Object} report - The report object containing thresholds, checks, metrics, groups, and scenarios.
 * @returns {string} A formatted summary of the test results.
 */
function generateTextSummary(data, options, report) {
	var mergedOpts = Object.assign({}, defaultOptions, data.options, options)
	var lines = []

	// TODO: move all of these functions into an object with methods?
	/**
	 * Decorates text with ANSI color codes.
	 *
	 * @param text
	 * @param _
	 * @returns {*}
	 */
	let decorate = function (text, _) {
		return text
	}
	if (mergedOpts.enableColors) {
		decorate = function (text, color /*, ...rest*/) {
			var result = '\x1b[' + color
			for (var i = 2; i < arguments.length; i++) {
				result += ';' + arguments[i]
			}
			return result + 'm' + text + '\x1b[0m'
		}
	}

	const ANSI_CODES = {
		reset: "\x1b[0m",

		// Standard Colors
		black: "\x1b[30m",
		red: "\x1b[31m",
		green: "\x1b[32m",
		yellow: "\x1b[33m",
		blue: "\x1b[34m",
		magenta: "\x1b[35m",
		cyan: "\x1b[36m",
		white: "\x1b[37m",

		// Bright Colors
		brightBlack: "\x1b[90m",
		brightRed: "\x1b[91m",
		brightGreen: "\x1b[92m",
		brightYellow: "\x1b[93m",
		brightBlue: "\x1b[94m",
		brightMagenta: "\x1b[95m",
		brightCyan: "\x1b[96m",
		brightWhite: "\x1b[97m",

		// Dark Colors
		darkGrey: "\x1b[90m",
	};

	const BOLD = '\u001b[1m'
	const RESET = ANSI_CODES.reset;
	const boldify = (text) => BOLD + text + RESET

	const defaultIndent = ' '
	const metricGroupIndent = '  '

	/**
	 * Displays a metrics block name (section heading).
	 *
	 * @param {string} sectionName - The section name (e.g., "checks", "http_req_duration").
	 * @param {Partial<DisplayOptions>} opts - Display options.
	 */
	const displayMetricsBlockName = (sectionName, opts) => {
		let bold = true;
		if (opts && opts.bold === false) {
			bold = false
		}

		let normalizedSectionName = sectionName.toUpperCase()

		if (bold) {
			normalizedSectionName = boldify(normalizedSectionName)
		}

		let indent = '    '
		if (opts && opts.metricsBlockIndent) {
			indent += opts.metricsBlockIndent
		}
		lines.push(indent + normalizedSectionName)
	}

	/**
	 * Displays a block of metrics with the given options.
	 *
	 * @param {Object[]} sectionMetrics - The metrics to display.
	 * @param {Partial<DisplayOptions>} opts - Display options.
	 */
	const displayMetricsBlock = (sectionMetrics, opts) => {
		const summarizeOpts = Object.assign({}, mergedOpts, opts)
		Array.prototype.push.apply(lines, summarizeMetrics(summarizeOpts, {metrics: sectionMetrics}, decorate))
		lines.push('')
	}

	/**
	 * Displays checks within a certain context (indentation, etc.).
	 *
	 * @param {Object} checks - Checks data, containing `metrics` and `ordered_checks`.
	 * @param {Partial<DisplayOptions>} [opts={indent: ''}] - Options including indentation.
	 */
	const displayChecks = (checks, opts = {indent: ''}) => {
		if (checks === undefined || checks === null) {
			return
		}
		displayMetricsBlock(checks.metrics, {...opts, indent: opts.indent + defaultIndent, sortByName: false})
		for (var i = 0; i < checks.ordered_checks.length; i++) {
			lines.push(summarizeCheck(metricGroupIndent + metricGroupIndent + opts.indent, checks.ordered_checks[i], decorate))
		}
		if (checks.ordered_checks.length > 0) {
			lines.push('')
		}
	}

	/**
	 * Displays thresholds and their satisfaction status.
	 *
	 * @param {Record<string, {metric: ReportMetric, thresholds: Threshold[]}>} thresholds - Threshold data.
	 * @param {Partial<DisplayOptions>} [opts={indent: ''}] - Display options.
	 */
	const displayThresholds = (thresholds, opts = {indent: ''}) => {
		if (thresholds === undefined || thresholds === null) {
			return
		}

		lines.push(metricGroupIndent + groupPrefix + defaultIndent + boldify('THRESHOLDS') + '\n')

		const mergedOpts = Object.assign({}, defaultOptions, data.options, options)

		let metrics = {};
		forEach(thresholds, (_, threshold) => {
			metrics[threshold.metric.name] = {...threshold.metric, thresholds: threshold.thresholds}
		});

		Array.prototype.push.apply(lines, summarizeMetricsWithThresholds(
			{...mergedOpts, indent: mergedOpts.indent + defaultIndent},
			{metrics},
			decorate),
		)
		lines.push('')
	};

	// THRESHOLDS
	displayThresholds(report.thresholds)

	// TOTAL RESULTS
	lines.push(metricGroupIndent + groupPrefix + defaultIndent + boldify('TOTAL RESULTS') + '\n')

	// CHECKS
	displayChecks(report.checks)

	// METRICS
	forEach(report.metrics, (sectionName, sectionMetrics) => {
		// If there are no metrics in this section, skip it
		if (Object.keys(sectionMetrics).length === 0) {
			return
		}

		displayMetricsBlockName(sectionName)
		displayMetricsBlock(sectionMetrics)
	})
	// END OF TOTAL RESULTS

	// GROUPS
	const summarize = (prefix, indent) => {
		return (groupName, groupData) => {
			lines.push(metricGroupIndent + indent + prefix + defaultIndent + boldify(`GROUP: ${groupName}`) + '\n')
			displayChecks(groupData.checks, {indent: indent})
			forEach(groupData.metrics, (sectionName, sectionMetrics) => {
				// If there are no metrics in this section, skip it
				if (Object.keys(sectionMetrics).length === 0) {
					return
				}

				displayMetricsBlockName(sectionName, {metricsBlockIndent: indent})
				displayMetricsBlock(sectionMetrics, {indent: indent + defaultIndent})
			})
			if (groupData.groups !== undefined) {
				forEach(groupData.groups, summarize(detailsPrefix, indent + metricGroupIndent));
			}
		}
	}

	const summarizeNestedGroups = (groupName, groupData) => {
		lines.push(metricGroupIndent + groupPrefix + ' ' + boldify(`GROUP: ${groupName}`) + '\n')
		forEach(groupData.metrics, (sectionName, sectionMetrics) => {
			// If there are no metrics in this section, skip it
			if (Object.keys(sectionMetrics).length === 0) {
				return
			}

			displayMetricsBlockName(sectionName)
			displayMetricsBlock(sectionMetrics)
		})
		if (groupData.groups !== undefined) {
			forEach(groupData.groups, summarizeNestedGroups);
		}
	}

	if (report.groups !== undefined) {
		forEach(report.groups, summarize(groupPrefix, defaultIndent));
	}

	// SCENARIOS
	if (report.scenarios !== undefined) {
		forEach(report.scenarios, (scenarioName, scenarioData) => {
			lines.push(metricGroupIndent + groupPrefix + defaultIndent + boldify(`SCENARIO: ${scenarioName}`) + '\n')
			displayChecks(scenarioData.checks)
			forEach(scenarioData.metrics, (sectionName, sectionMetrics) => {
				// If there are no metrics in this section, skip it
				if (Object.keys(sectionMetrics).length === 0) {
					return
				}

				displayMetricsBlockName(sectionName)
				displayMetricsBlock(sectionMetrics)
			})
			if (scenarioData.groups !== undefined) {
				forEach(scenarioData.groups, summarize(detailsPrefix, metricGroupIndent));
			}
		})
	}

	return lines.join('\n')
}

exports.humanizeValue = humanizeValue
exports.textSummary = generateTextSummary
