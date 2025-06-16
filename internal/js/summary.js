/**
 * This file contains code used to generate a textual summary of tests results, as displayed
 * in the user's terminal at the end of a k6 test run, also known as "end of test summary".
 *
 * The main entry point is the `generateTextSummary` function, which takes the test data as well as a report
 * object containing results for checks, metrics, thresholds, groups, and scenarios, and returns a formatted
 * string summarizing the test results, ready to be written to the terminal.
 *
 * For convenience, the file also exports the `humanizeValue` function.
 */
exports.humanizeValue = humanizeValue;
exports.textSummary = generateTextSummary;

/**
 * Generates a textual summary of test results, including checks, metrics, thresholds, groups, and scenarios.
 *
 * @param {Report} report - The report object containing thresholds, checks, metrics, groups, and scenarios.
 * @param {Object} options - Additional options that override defaults.
 * @returns {string} A formatted summary of the test results.
 */
function generateTextSummary(report, options) {
	const mergedOpts = Object.assign({}, defaultOptions, options);

	// Create a render context holding information such as indentation level to apply
	const context = new RenderContext(0);

	// Create a formatter with default settings (colors enabled)
	const formatter = new ANSIFormatter(mergedOpts);

	const reportGenerator = new TestReportGenerator(
		formatter,
		context,
		mergedOpts,
	);

	return reportGenerator.generate(report);
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
	if (metric.type === 'rate') {
		// Truncate instead of round when decreasing precision to 2 decimal places
		return (Math.trunc(val * 100 * 100) / 100).toFixed(2) + '%';
	}

	switch (metric.contains) {
		case 'data':
			return humanizeBytes(val);
		case 'time':
			return humanizeDuration(val, timeUnit);
		default:
			return toFixedNoTrailingZeros(val, 6);
	}
}

/**
 * @typedef {Object} Report
 * @property {Record<string, ReportThreshold>} thresholds - The thresholds report.
 * @property {ReportMetrics} metrics - The metrics report.
 * @property {Record<string, ReportGroup>} groups - The groups report.
 * @property {Record<string, ReportGroup>} scenarios - The scenarios report.
 */

/**
 * @typedef {Object} ReportThreshold
 * @property {string} source - The threshold expression source.
 * @property {boolean} ok - Whether the threshold was satisfied or not.
 */

/**
 * @typedef {Object} ReportGroup
 * @property {ReportChecks} checks - The checks report.
 * @property {ReportMetrics} metrics - The metrics report.
 * @property {Record<string, ReportGroup>} groups - The nested groups report.
 */

/**
 * @typedef {Object} ReportMetric
 * @property {string} name - The name of the reported metric.
 * @property {"counter"|"gauge"|"rate"|"trend"} type - The type of the metric.
 * @property {"time"|"data"|"default"} contains - The type of data contained in the metric
 * @property {Record<string, number>} values - Key-value pairs of metric statistics (e.g. min, max, avg).
 * @property {EngineThreshold[]} [thresholds] - Optional array of thresholds associated with this metric.
 */

/**
 * @typedef {Object} ReportMetrics
 * @property {Record<string, ReportMetric>} http - The HTTP metrics.
 * @property {Record<string, ReportMetric>} execution - The execution-related metrics.
 * @property {Record<string, ReportMetric>} network - The network-related metrics.
 * @property {Record<string, ReportMetric>} browser - The browser-related metrics.
 * @property {Record<string, ReportMetric>} webvitals - The web vitals metrics.
 * @property {Record<string, ReportMetric>} grpc - The grpc-related metrics.
 * @property {Record<string, ReportMetric>} websocket - The websocket-related metrics.
 * @property {Record<string, ReportMetric>} miscelaneous - The custom metrics.
 */

/**
 * @typedef {Object} ReportChecks
 * @property {Record<string, ReportMetric>} metrics - The metrics for checks.
 * @property {EngineCheck[]} ordered_checks - The ordered checks.
 */

/**
 * @typedef {Object} ReportMetricThresholds
 * @property {ReportMetric} metric - The metric object.
 * @property {ReportThreshold[]} thresholds - The thresholds for the metric.
 */

/**
 * @typedef {Object} ReportData
 * @property {Record<string, ReportMetric>} metrics - Collection of metrics keyed by their names.
 */

/**
 * @typedef {Object} EngineCheck
 * @property {string} id - The check ID.
 * @property {string} name - The check name.
 * @property {string} path - The check path.
 * @property {number} passes - The number of successful checks.
 * @property {number} fails - The number of failed checks.
 */

/**
 * @typedef {Object} EngineThreshold
 * @property {string} source - The threshold expression source.
 * @property {boolean} ok - Whether the threshold was satisfied or not.
 */

/**
 * @typedef {Object} Options
 * @property {boolean} [enableColors = true] - Whether to enable ANSI colors.
 * @property {string | null} [summaryTimeUnit = null] - The time unit for duration metrics.
 * @property {string[] | null} [summaryTrendStats = null] - The trend statistics to summarize.
 * @property {boolean} [sortByName = true] - Whether to sort metrics by name.
 */

/**
 * Generates a textual summary of test results, including checks, metrics, thresholds, groups, and scenarios.
 */
class TestReportGenerator {
	/**
	 * Constructs a TestReportGenerator with a specified formatter
	 *
	 * @param {ANSIFormatter} formatter - The ANSI formatter to use for text decoration.
	 * @param {RenderContext} renderContext - The render context to use for text rendering.
	 * @param {Options} [options = {}]
	 */
	constructor(formatter, renderContext, options = {}) {
		this.formatter = formatter;
		this.renderContext = renderContext;
		this.options = options;
	}

	/**
	 * Generates a textual summary of test results, including checks, metrics, thresholds, groups, and scenarios.
	 *
	 * @param {Report} report - The report object containing thresholds, checks, metrics, groups, and scenarios as provided by k6.
	 * @returns {string} - A formatted summary of the test results.
	 */
	generate(report) {
		const reportBuilder = new ReportBuilder(
			this.formatter,
			this.renderContext,
			this.options,
		);
		return reportBuilder
			.addThresholds(report.thresholds)
			.addTotalResults(report)
			.addGroups(report.groups)
			.addScenarios(report.scenarios)
			.build();
	}
}

/**
 * Exposes methods for generating a textual summary of test results.
 */
class ReportBuilder {
	/**
	 * Creates a new ReportBuilder with a specified formatter and options.
	 *
	 * @param {ANSIFormatter} formatter - The ANSI formatter to use for text decoration.
	 * @param {RenderContext} renderContext - The render context to use for text rendering.
	 * @param options
	 */
	constructor(formatter, renderContext, options) {
		this.formatter = formatter;
		this.renderContext = renderContext;
		this.options = options;
		this.sections = [];
	}

	/**
	 * Adds a thresholds section to the report.
	 *
	 * @param {Record<string, ReportThreshold>} thresholds - The thresholds to add to the report.
	 * @returns {ReportBuilder}
	 */
	addThresholds(thresholds) {
		if (!thresholds || Object.keys(thresholds).length === 0) return this;

		this.sections.push({
			title: 'THRESHOLDS',
			content: this._renderThresholds(thresholds),
		});
		return this;
	}

	/**
	 * Adds a total results section to the report.
	 *
	 * @param {Report} report - The report object containing thresholds, checks, metrics, groups, and scenarios as provided by k6.
	 * @returns {ReportBuilder}
	 */
	addTotalResults(report) {
		this.sections.push({
			title: 'TOTAL RESULTS',
			content: [
				...this._renderChecks(report.checks),
				...this._renderMetrics(report.metrics),
			],
		});
		return this;
	}

	/**
	 * Adds groups sections to the report.
	 *
	 * @param {Record<string, ReportGroup>} groups - The groups to add to the report.
	 * @returns {ReportBuilder}
	 */
	addGroups(groups) {
		if (!groups) return this;

		Object.entries(groups)
			.forEach(([groupName, groupData]) => {
				this.sections.push({
					title: `GROUP: ${groupName}`,
					content: this._renderGroupContent(groupData),
				});
			});
		return this;
	}

	/**
	 * Adds scenarios sections to the report.
	 *
	 * @param {Record<string, ReportGroup>} scenarios - The scenarios to add to the report.
	 * @returns {ReportBuilder}
	 */
	addScenarios(scenarios) {
		if (!scenarios) return this;

		Object.entries(scenarios)
			.sort(([a], [b]) => a.localeCompare(b))
			.forEach(([scenarioName, scenarioData]) => {
				this.sections.push({
					title: `SCENARIO: ${scenarioName}`,
					content: this._renderScenarioContent(scenarioData),
				});
			});
		return this;
	}

	/**
	 * Builds the final report by concatenating all sections together, resulting
	 * in a formatted string ready to be printed to the terminal.
	 *
	 * @returns {string}
	 */
	build() {
		return this.sections
			.map((section) => [
				renderTitle(section.title, this.formatter, this.renderContext),
				...section.content,
				'\n',
			])
			.flat()
			.reduce((acc, curr) => {
				return (curr === '\n') ? acc + curr : acc + '\n' + curr;
			}, '');
	}

	/**
	 * @param {Record<string, ReportThreshold>} thresholds
	 * @param {RenderContext} [renderContext]
	 * @returns {string[]}
	 * @private
	 */
	_renderThresholds(thresholds, renderContext) {
		// The thresholds list should be indent one level higher than the title
		renderContext = renderContext || this.renderContext;
		renderContext = renderContext.indentedContext(1);

		// Implement threshold rendering logic
		return renderThresholds(
			this._processThresholds(thresholds),
			this.formatter,
			renderContext,
			this.options,
		);
	}

	/**
	 * @param checks
	 * @param {RenderContext} [renderContext] - The render context to use for text rendering.
	 * @returns {string[]}
	 * @private
	 */
	_renderChecks(checks, renderContext) {
		renderContext = renderContext || this.renderContext;

		return checks
			? [...renderChecks(checks, this.formatter, renderContext, this.options), '\n']
			: [];
	}

	/**
	 * @param {ReportMetrics} metrics - The metrics to render.
	 * @param {RenderContext} [renderContext] - The render context to use for text rendering.
	 * @returns {string[]}
	 * @private
	 */
	_renderMetrics(metrics, renderContext) {
		renderContext = renderContext || this.renderContext;
		renderContext = renderContext.indentedContext(1);

		// Collect all metrics into a single object, so we can precompute all formatting information
		const allMetrics = Object.entries(metrics).reduce((acc, [_, metrics]) => {
			Object.assign(acc, metrics);
			return acc;
		}, {});

		// Precompute all formatting information
		const summaryInfo = computeSummaryInfo(
			allMetrics,
			renderContext,
			this.options,
		);

		// Implement metrics rendering logic
		return Object.entries(metrics)
			.filter(
				([_, sectionMetrics]) => Object.keys(sectionMetrics).length > 0,
			)
			.reduce(
				(acc, [sectionName, sectionMetrics]) => (sectionName === "custom")
					? [[sectionName, sectionMetrics], ...acc]
					: [...acc, [sectionName, sectionMetrics]]
				, [])
			.flatMap(([sectionName, sectionMetrics]) => [
				renderContext.indent(
					this.formatter.boldify(sectionName.toUpperCase()),
				),
				...renderMetrics(
					sectionMetrics,
					summaryInfo,
					this.formatter,
					renderContext,
					this.options,
				),
				'\n',
			]);
	}

	/**
	 * @param {ReportGroup} group - The group data to render.
	 * @param {RenderContext} [renderContext] - The render context to use for text rendering.
	 * @returns {string[]}
	 * @private
	 */
	_renderGroupContent(group, renderContext) {
		renderContext = renderContext || this.renderContext;

		// Implement group content rendering
		return [
			...this._renderChecks(group.checks, renderContext),
			...this._renderMetrics(group.metrics, renderContext),
			...(group.groups ? this._renderNestedGroups(group.groups, renderContext) : []),
		];
	}

	/**
	 * @param {ReportGroup} scenarioData - The scenario data to render.
	 * @param {RenderContext} [renderContext] - The render context to use for text rendering.
	 * @returns {string[]}
	 * @private
	 */
	_renderScenarioContent(scenarioData, renderContext) {
		renderContext = renderContext || this.renderContext;

		// Similar to group content rendering
		return [
			...this._renderChecks(scenarioData.checks, renderContext),
			...this._renderMetrics(scenarioData.metrics, renderContext),
			...(scenarioData.groups
				? this._renderNestedGroups(scenarioData.groups, renderContext)
				: []),
		];
	}

	/**
	 * @param {Record<string, ReportGroup>} groups - The nested groups data to render.
	 * @param {RenderContext} [renderContext] - The render context to use for text rendering.
	 * @returns {string[]}
	 * @private
	 */
	_renderNestedGroups(groups, renderContext) {
		renderContext = renderContext || this.renderContext;
		renderContext = renderContext.indentedContext(1);

		// Render nested groups recursively
		return Object.entries(groups)
			.flatMap(([groupName, groupData]) => [
				renderTitle(`GROUP: ${groupName}`, this.formatter, renderContext, {
					prefix: subtitlePrefix,
					suffix: '\n',
				}),
				...this._renderGroupContent(groupData, renderContext),
			]);
	}

	// Private rendering methods
	/**
	 *
	 * @param {ReportMetricThresholds} thresholds - The thresholds data to render.
	 * @returns {Record<string, ReportMetric>}
	 * @private
	 */
	_processThresholds(thresholds) {
		// Transform thresholds into a format suitable for rendering
		const metrics = {};
		Object.values(thresholds).forEach((threshold) => {
			metrics[threshold.metric.name] = {
				...threshold.metric,
				thresholds: threshold.thresholds,
			};
		});
		return metrics;
	}
}

/**
 * RenderContext is a helper class that provides methods for rendering text
 * with indentation.
 *
 * It is used to keep track of the current indentation level and provide
 * methods for rendering text with the correct indentation.
 *
 * It also facilitates the creation of new RenderContext instances with
 * different indentation levels. That way the indentation level can be
 * easily adjusted relatively to a parent indentation level without having
 * to manage some dedicated state manually.
 */
class RenderContext {
	constructor(baseIndentationLevel = 0) {
		this.baseIndentationLevel = baseIndentationLevel;
	}

	/**
	 * Returns a string of spaces for a given indentation level.
	 *
	 * @param {number} [level]
	 * @returns {string}
	 */
	indentLevel(level = 1) {
		return ' '.repeat((this.baseIndentationLevel + level) * 2);
	}

	/**
	 * @param {string} text - The text to indent.
	 * @param {number} [level]
	 * @returns {string}
	 */
	indent(text, level = 1) {
		return this.indentLevel(level) + text;
	}

	/**
	 * indentedContext returns a new RenderContext with an incremented base indentation level.
	 *
	 * This allows to easily obtain a new RenderContext from a parent one with an
	 * increased indentation level.
	 *
	 * @param {number} increment - The increment to apply to the base indentation level.
	 * @returns {RenderContext}
	 */
	indentedContext(increment = 1) {
		return new RenderContext(this.baseIndentationLevel + increment);
	}
}

/**
 * ANSIFormatter provides methods for decorating text with ANSI color and style codes.
 */
class ANSIFormatter {
	/**
	 * Constructs an ANSIFormatter with configurable color and styling options
	 *
	 * @param {Object} options - Configuration options for formatting
	 * @param {boolean} [options.enableColors=true] - Whether to enable color output
	 */
	constructor(options = {}) {
		this.options = {
			enableColors: true,
			...options,
		};
	}

	/**
	 * Decorates text with ANSI color and style.
	 *
	 * @param {string} text - The text to decorate.
	 * @param {ANSIColor} color - The ANSI color to apply.
	 * @param {...ANSIStyle} styles - optional additional styles to apply.
	 * @returns {string} - Decorated text, or plain text if colors are disabled.
	 */
	decorate(text, color, ...styles) {
		if (!this.options.enableColors) {
			return text;
		}

		const colorCode = ANSIColors[color] || ANSIColors.white;
		const styleCodes = styles
			.map((style) => ANSIStyles[style])
			.filter(Boolean);

		const fullCodes = styleCodes.length
			? [...styleCodes, colorCode].join(';')
			: colorCode;

		const fullSequence = `\x1b[${fullCodes}m`;

		return `${fullSequence}${text}\x1b[0m`;
	}

	/**
	 * Applies bold styling to text
	 *
	 * @param {string} text - Text to make bold
	 * @returns {string} Bold text
	 */
	boldify(text) {
		if (!this.options.enableColors) {
			return text;
		}
		return `\u001b[1m${text}\x1b[0m`;
	}
}

/**
 * ANSIColor maps ANSI color names to their respective escape codes.
 *
 * @typedef {'reset'|'black'|'red'|'green'|'yellow'|'blue'|'magenta'|'cyan'|
 *           'white'|'brightRed'|'brightGreen'|'brightYellow'} ANSIColor
 *
 * @typedef {Record<ANSIColor, string>} ANSIColors
 */
const ANSIColors = {
	reset: '\x1b[0m',

	// Standard Colors
	black: '30',
	red: '31',
	green: '32',
	yellow: '33',
	blue: '34',
	magenta: '35',
	cyan: '36',
	white: '37',

	// Bright Colors
	brightRed: '91',
	brightGreen: '92',
	brightYellow: '93',
};

/**
 * ANSIStyle maps ANSI style names to their respective escape codes.
 *
 * @typedef {'bold' | 'faint' | 'underline' | 'reversed'} ANSIStyle
 *
 * @typedef {Record<ANSIStyle, string>} ANSIStyles
 */
const ANSIStyles = {
	bold: '1',
	faint: '2',
	underline: '4',
	reversed: '7',
};

/**
 * Renders a section title with a specified formatter, indentation level, and options.
 *
 * For example, a bold section title at first indentation level with a block prefix and newline suffix:
 *  █ THRESHOLDS
 *
 * @param {string} title - The section title to render.
 * @param {ANSIFormatter} formatter - The ANSI formatter to use for text decoration.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @param {Options & Object} options - Additional options for rendering the section title.
 * @param {string} [options.prefix=titlePrefix] - The prefix to use for the section title.
 * @param {string} [options.suffix='\n'] - The suffix to use for the section title.
 * @returns {string} - The formatted section title.
 */
function renderTitle(
	title,
	formatter,
	renderContext,
	options = {prefix: titlePrefix, suffix: '\n'},
) {
	return renderContext.indent(
		`${options.prefix} ${formatter.boldify(title)} ${options.suffix || ''}`,
	);
}

/**
 * Renders a single check into a formatted line ready for output.
 *
 * @param {EngineCheck} check - The check object with name, passes and fails
 * @param {ANSIFormatter} formatter - ANSI formatter used for decorating text.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @returns {string} - A formatted line summarizing the check.
 */
function renderCheck(check, formatter, renderContext) {
	// If the check was successful, immediately render a green line indicating success
	if (check.fails === 0) {
		return renderContext.indent(
			formatter.decorate(successMark + ' ' + check.name, 'green'),
		);
	}

	// Other we want to display both the check name and the percentage of successful checks
	// in red, along with the number of passes and fails.
	const successfulPct = Math.floor(
		(100 * check.passes) / (check.passes + check.fails),
	);

	const checkName = formatter.decorate(failMark + ' ' + check.name, 'red');
	const results = formatter.decorate(
		subtitlePrefix +
		'  ' +
		successfulPct +
		'% — ' +
		successMark +
		' ' +
		check.passes +
		' / ' +
		failMark +
		' ' +
		check.fails,
		'red',
	);

	return (
		renderContext.indent(checkName) +
		'\n' +
		renderContext.indent(results, renderContext.baseIndentationLevel + 1)
	);
}

/**
 * Renders checks into a formatted set of lines ready for display in the terminal.
 *
 * @param {ReportChecks} checks
 * @param formatter
 * @param {RenderContext} renderContext
 * @param options
 * @returns {*[]}
 */
function renderChecks(checks, formatter, renderContext, options = {}) {
	// If no checks exist, return empty array
	if (!checks || !checks.ordered_checks) {
		return [];
	}

	// Add indentation to the render context for checks
	renderContext = renderContext.indentedContext(1);

	const {showPassedChecks = true, showFailedChecks = true} = options;

	// Process each check and filter based on options
	const renderedChecks = checks.ordered_checks
		.filter((check) => {
			// Filter logic for passed/failed checks
			if (check.fails === 0 && !showPassedChecks) return false;
			return !(check.fails > 0 && !showFailedChecks);
		})
		.map((check) => renderCheck(check, formatter, renderContext));

	// Precompute all formatting information
	const summaryInfo = computeSummaryInfo(
		checks.metrics,
		renderContext,
		options,
	);

	// Render metrics for checks if they exist
	const checkMetrics = checks.metrics
		? renderMetrics(checks.metrics, summaryInfo, formatter, renderContext, {
			...options,
			sortByName: false,
		})
		: [];

	// Combine metrics and checks
	return [...checkMetrics, '\n', ...renderedChecks];
}

/**
 * Summarizes metrics into an array of formatted lines ready to be printed to stdout.
 *
 * @param {Record<string, ReportMetric>} metrics - The data object containing metrics.
 * @param {SummaryInfo} summaryInfo - An object containing summary information such as maximum name width and trend columns.
 * @param {ANSIFormatter} formatter - An ANSIFormatter function for ANSI colors.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @param {Options} options - Display options merged with defaultOptions.
 * @returns {string[]}
 */
function renderMetrics(
	metrics,
	summaryInfo,
	formatter,
	renderContext,
	options,
) {
	// Extract all metric names
	let metricNames = Object.keys(metrics);

	// If sorting by name is required, do it now
	if (options.sortByName) {
		metricNames = sortMetricsByName(metricNames);
	}

	// Format each metric line
	return metricNames.map((name) => {
		const metric = metrics[name];
		return renderMetricLine(
			name,
			metric,
			summaryInfo,
			options,
			formatter,
			renderContext,
		);
	});
}

/**
 * Renders each thresholds results into a formatted set of lines ready for display in the terminal.
 *
 * @param {Record<string, ReportMetric>} metrics - The data object containing metrics.
 * @param {ANSIFormatter} formatter - ANSI formatter used for decorating text.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @param {Object} options - Options merged with defaults.
 * @returns {string[]} - Array of formatted lines including threshold statuses.
 */
function renderThresholds(metrics, formatter, renderContext, options) {
	// Extract and optionally sort metric names
	let metricNames = Object.keys(metrics);
	if (options.sortByName) {
		metricNames = sortMetricsByName(metricNames);
	}

	// Precompute all formatting information
	const summaryInfo = computeSummaryInfo(
		metrics,
		renderContext,
		options,
	);

	// Format each threshold line by preparing each metric affected by a threshold, as
	// well as the thresholds results for each expression.
	const result = [];
	for (const name of metricNames) {
		const parentName = name.split('{', 1)[0];
		const isSubmetric = name.length > parentName.length;
		const parentMetricExists = !!metrics[parentName];

		const innerContext = (isSubmetric && parentMetricExists)
			? renderContext.indentedContext()
			: renderContext;

		const line = renderMetricNameForThresholds(
			name,
			parentName,
			isSubmetric,
			parentMetricExists,
			innerContext
		);
		result.push(line);

		const metric = metrics[name];
		if (metric.thresholds) {
			const thresholdLines = renderThresholdResults(
				metric,
				summaryInfo,
				formatter,
				innerContext,
			);
			result.push(...thresholdLines, '\n');
		}
	}

	return result
}

/**
 * Renders each threshold result into a formatted set of lines ready for display in the terminal.
 *
 * @param {ReportMetric} metric - The metric with the thresholds to render.
 * @param {SummaryInfo} summaryInfo - An object containing summary information such as maximum name width and trend columns.
 * @param {ANSIFormatter} formatter - ANSIFormatter used for decorating text.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @returns {string[]} - An array of formatted lines including threshold statuses.
 */
function renderThresholdResults(
	metric,
	summaryInfo,
	formatter,
	renderContext,
) {
	const lines = [];

	forEach(metric.thresholds, (_, threshold) => {
		const isSatisfied = threshold.ok;
		const statusText = isSatisfied
			? formatter.decorate(successMark, 'green')
			: formatter.decorate(failMark, 'red');

		const sourceText = formatter.decorate(
			`'${threshold.source.trim()}'`,
			'white',
		);

		const metricValueText = renderMetricValueForThresholds(
			metric,
			threshold,
			summaryInfo,
			formatter,
		)

		// Here we push a line describing the threshold's result
		lines.push(
			renderContext.indent([statusText, sourceText, metricValueText].join(' ')),
		);
	});

	return lines;
}

/**
 * Renders a metric line into a formatted string for display.
 *
 * @param {string} name - The name of the metric.
 * @param {ReportMetric} metric - The metric object containing details about the metric.
 * @param {SummaryInfo} info - An object containing summary information such as maximum name width and trend columns.
 * @param {Options} options - Configuration options for summarizing metrics.
 * @param {ANSIFormatter} formatter - A function to apply ANSI colors to text.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @returns {string} - The formatted metric line.
 */
function renderMetricLine(
	name,
	metric,
	info,
	options,
	formatter,
	renderContext,
) {
	const {maxNameWidth} = info;

	const displayedName = renderMetricDisplayName(name);
	const fmtIndent = renderContext.indentLevel();

	// Compute the trailing dots:
	// Use `3` as a spacing offset as per original code.
	const dotsCount =
		maxNameWidth - strWidth(displayedName) - strWidth(fmtIndent) + 3;
	const dottedName =
		displayedName +
		formatter.decorate('.'.repeat(dotsCount) + ':', 'white', 'faint');

	const dataPart =
		metric.type === 'trend'
			? renderTrendData(name, info, formatter)
			: renderNonTrendData(name, info, formatter);

	return renderContext.indent(dottedName + ' ' + dataPart);
}

/**
 * Formats a metric or submetric line for the thresholds' section output.
 *
 * @param {string} name - The name of the metric
 * @param {string} parentName - The name of the parent metric
 * @param {boolean} isSubmetric - Whether the metric is a submetric
 * @param {boolean} parentMetricExists - In case of submetric, whether the parent metric exists
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @returns {string} - The metric name report line
 */
function renderMetricNameForThresholds(
	name,
	parentName,
	isSubmetric,
	parentMetricExists,
	renderContext,
) {
	// If it's a parent metric, or it's a submetric,
	// which parent metric is not included in results, we just print the name.
	if (!isSubmetric || !parentMetricExists) {
		return renderContext.indent(name);
	}

	// Otherwise, we only print the labels.
	return renderContext.indent(name.substring(parentName.length));
}

/**
 * Formats the metric's value for the thresholds' section output.
 *
 * @param {ReportMetric} metric - The metric for which value will be rendered.
 * @param {EngineThreshold} threshold - The threshold to use for rendering.
 * @param {SummaryInfo} info - An object containing summary information such as maximum name width and trend columns.
 * @param {ANSIFormatter} formatter - ANSIFormatter used for decorating text.
 * @returns {string} - The metric's value line in the form: `{agg}={value}`
 */
function renderMetricValueForThresholds(
	metric,
	threshold,
	info,
	formatter,
) {
	const {trendStats, trendCols, trendKeys, nonTrendValues, nonTrendExtras} = info;
	const thresholdAgg = threshold.source.split(/[=><]/)[0].trim();

	let value;
	switch (metric.type) {
		case 'trend':
			const trendStatIndex = trendStats.indexOf(thresholdAgg);
			value = (trendStatIndex !== -1)
				? trendCols[metric.name]?.[trendStatIndex]
				: trendKeys[metric.name]?.[thresholdAgg];
			break;
		case 'counter':
			value = (thresholdAgg === 'count')
				? nonTrendValues[metric.name]
				: nonTrendExtras[metric.name][0];
			break;
		default:
			value = nonTrendValues[metric.name];
	}

	return [
		formatter.decorate(thresholdAgg, 'white'),
		formatter.decorate(value, 'cyan')
	].join('=');
}

/**
 * Format data for trend metrics.
 */
function renderTrendData(name, info, formatter) {
	const {trendStats, trendCols, trendColMaxLens} = info;
	const cols = trendCols[name];

	return cols
		.map((col, i) => {
			const statName = trendStats[i];
			const padding = ' '.repeat(trendColMaxLens[i] - strWidth(col));
			return statName + '=' + formatter.decorate(col, 'cyan') + padding;
		})
		.join(' ');
}

/**
 * Format data for non-trend metrics.
 *
 * @param {string} name - The metric name.
 * @param {SummaryInfo} info - The summary information object.
 * @param {ANSIFormatter} formatter - A decoration function for ANSI colors.
 */
function renderNonTrendData(name, info, formatter) {
	const {
		nonTrendValues,
		nonTrendExtras,
		maxNonTrendValueLen,
		nonTrendExtraMaxLens,
	} = info;

	const value = nonTrendValues[name];
	const extras = nonTrendExtras[name] || [];

	let result = formatter.decorate(value, 'cyan');
	result += ' '.repeat(maxNonTrendValueLen - strWidth(value));

	if (extras.length === 1) {
		// Single extra value
		result += ' ' + formatter.decorate(extras[0], 'cyan', 'faint');
	} else if (extras.length > 1) {
		// Multiple extras need their own spacing
		const parts = extras.map((val, i) => {
			const extraSpace = ' '.repeat(
				nonTrendExtraMaxLens[i] - strWidth(val),
			);
			return formatter.decorate(val, 'cyan', 'faint') + extraSpace;
		});
		result += ' ' + parts.join(' ');
	}

	return result;
}

/**
 *
 * @param {number} value
 * @param {string} stat
 * @param {ReportMetric} metric
 * @param {Options} options
 * @returns {string}
 */
function renderTrendValue(value, stat, metric, options) {
	if (stat === 'count') {
		return value.toString();
	}
	return humanizeValue(value, metric, options.summaryTimeUnit);
}

/**
 * Compute all necessary formatting information such as maximum lengths, trend columns and non-trend values for each
 * metric.
 *
 * @typedef {Object} SummaryInfo
 * @property {number} maxNameWidth - The maximum width of the metric names.
 * @property {Object} nonTrendValues - The non-trend metric values.
 * @property {Object} nonTrendExtras - The non-trend metric extras.
 * @property {Object} trendCols - The trend columns.
 * @property {Object} trendKeys - The trend keys (values that aren't included within `trendStats`).
 * @property {number[]} trendColMaxLens - The trend column maximum lengths.
 * @property {number} numTrendColumns - The number of trend columns.
 * @property {string[]} trendStats - The trend statistics.
 * @property {number} maxNonTrendValueLen - The maximum non-trend value length.
 * @property {number[]} nonTrendExtraMaxLens - The non-trend extra maximum lengths.
 *
 * @param {Record<string, ReportMetric>} metrics - The data object containing metrics.
 * @param {RenderContext} renderContext - The render context to use for text rendering.
 * @param {Options} options - Display options merged with defaultOptions.
 * @returns {SummaryInfo}
 */
function computeSummaryInfo(metrics, renderContext, options) {
	const trendStats = options.summaryTrendStats;
	const numTrendColumns = trendStats.length;

	const nonTrendValues = {};
	const nonTrendExtras = {};
	const trendCols = {};

	// While "trendCols" contain the values for each "trendStats" aggregation (e.g. p(90) as a sorted array,
	// "trendKeys" is used to store specific aggregation values that aren't part of "trendStats"; mainly for thresholds.
	const trendKeys = {};

	let maxNameWidth = 0;
	let maxNonTrendValueLen = 0;
	let nonTrendExtraMaxLens = []; // FIXME: "lens"?

	// Initialize tracking arrays for trend widths
	const trendColMaxLens = new Array(numTrendColumns).fill(0);

	let metricNames = Object.keys(metrics);
	// If sorting by name is required, do it now
	if (options.sortByName) {
		metricNames = sortMetricsByName(metricNames);
	}

	for (const name of metricNames) {
		const metric = metrics[name];
		const displayName = renderContext.indent(
			name + renderMetricDisplayName(name),
		);
		maxNameWidth = Math.max(maxNameWidth, strWidth(displayName));

		if (metric.type === 'trend') {
			const keys = Object.keys(metric.values).reduce((acc, key) => {
				if (!trendStats.includes(key)) {
					acc[key] = renderTrendValue(metric.values[key], key, metric, options);
				}
				return acc;
			}, {});

			const cols = trendStats.map((stat) =>
				renderTrendValue(metric.values[stat], stat, metric, options),
			);

			// Compute max column widths
			cols.forEach((col, index) => {
				trendColMaxLens[index] = Math.max(
					trendColMaxLens[index],
					strWidth(col),
				);
			});
			trendCols[name] = cols;
			trendKeys[name] = keys;
		} else {
			const values = nonTrendMetricValueForSum(
				metric,
				options.summaryTimeUnit,
			);
			const mainValue = values[0]; // FIXME (@oleiade) we should assert that the index exists here
			nonTrendValues[name] = mainValue;
			maxNonTrendValueLen = Math.max(
				maxNonTrendValueLen,
				strWidth(mainValue),
			);

			// FIXME (@oleiade): what the fuck is an extra, really?
			const extras = values.slice(1);
			nonTrendExtras[name] = extras;
			extras.forEach((value, index) => {
				const width = strWidth(value);
				if (
					nonTrendExtraMaxLens[index] === undefined ||
					width > nonTrendExtraMaxLens[index]
				) {
					nonTrendExtraMaxLens[index] = width;
				}
			});
		}
	}

	return {
		maxNameWidth,
		nonTrendValues,
		nonTrendExtras,
		trendCols,
		trendColMaxLens,
		trendKeys,
		numTrendColumns,
		trendStats,
		maxNonTrendValueLen,
		nonTrendExtraMaxLens,
	};
}

/**
 * Sorts metrics by name, keeping sub-metrics grouped with their parent metrics.
 *
 * @param {string[]} metricNames - The metric names to sort.
 * @returns {string[]} - The sorted metric names.
 */
function sortMetricsByName(metricNames) {
	metricNames.sort(function (lhsMetricName, rhsMetricName) {
		const lhsParent = lhsMetricName.split('{', 1)[0];
		const rhsParent = rhsMetricName.split('{', 1)[0];
		const result = lhsParent.localeCompare(rhsParent);
		if (result !== 0) {
			return result;
		}
		const lhsSub = lhsMetricName.substring(lhsParent.length);
		const rhsSub = rhsMetricName.substring(rhsParent.length);
		return lhsSub.localeCompare(rhsSub);
	});

	return metricNames;
}

/**
 * A simple iteration utility function for objects.
 *
 * @param {Object} obj - the object to iterate over
 * @param {(key: string, value: any) => (boolean|void)} callback - Callback invoked with (key, value)
 */
function forEach(obj, callback) {
	for (const key in obj) {
		if (obj.hasOwnProperty(key)) {
			if (callback(key, obj[key])) {
				break;
			}
		}
	}
}

const titlePrefix = '█';
const subtitlePrefix = '↳';
const successMark = '✓';
const failMark = '✗';
const defaultOptions = {
	indent: ' ',
	enableColors: true, // FIXME (@oleiade): we should ensure we respect this flag
	summaryTimeUnit: null,
	summaryTrendStats: null,
	sortByName: true,
};

/**
 * Compute the width of a string as displayed in a terminal, excluding ANSI codes, terminal
 * formatting, Unicode ligatures, etc.
 *
 * @param {string} s - The string to measure
 * @returns {number} The display width of the string
 */
function strWidth(s) {
	// TODO: determine if NFC or NFKD are not more appropriate? or just give up? https://hsivonen.fi/string-length/
	const data = s.normalize('NFKC'); // This used to be NFKD in Go, but this should be better
	let inEscSeq = false;
	let inLongEscSeq = false;
	let width = 0;
	for (const char of data) {
		if (char.done) {
			break;
		}

		// Skip over ANSI escape codes.
		if (char === '\x1b') {
			inEscSeq = true;
			continue;
		}
		if (inEscSeq && char === '[') {
			inLongEscSeq = true;
			continue;
		}
		if (
			inEscSeq &&
			inLongEscSeq &&
			char.charCodeAt(0) >= 0x40 &&
			char.charCodeAt(0) <= 0x7e
		) {
			inEscSeq = false;
			inLongEscSeq = false;
			continue;
		}
		if (
			inEscSeq &&
			!inLongEscSeq &&
			char.charCodeAt(0) >= 0x40 &&
			char.charCodeAt(0) <= 0x5f
		) {
			inEscSeq = false;
			continue;
		}

		if (!inEscSeq && !inLongEscSeq) {
			width++;
		}
	}
	return width;
}

/**
 * Extracts a display name for a metric, handling sub-metrics (e.g. "metric{sub}" -> "{ sub }").
 *
 * @param {string} name - The metric name.
 * @returns {string} - The display name
 */
function renderMetricDisplayName(name) {
	const subMetricPos = name.indexOf('{');
	if (subMetricPos >= 0) {
		return '  { ' + name.substring(subMetricPos + 1, name.length - 1) + ' }';
	}
	return name;
}

/**
 * Converts a number of bytes into a human-readable string with units.
 *
 * @param {number} bytes - The number of bytes.
 * @returns {string} A human-readable string (e.g. "10 kB").
 */
function humanizeBytes(bytes) {
	const units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
	const base = 1000;
	if (bytes < 10) {
		return bytes + ' B';
	}

	const e = Math.floor(Math.log(bytes) / Math.log(base));
	const suffix = units[e | 0];
	const val = Math.floor((bytes / Math.pow(base, e)) * 10 + 0.5) / 10;
	return val.toFixed(val < 10 ? 1 : 0) + ' ' + suffix;
}

const unitMap = {
	s: {unit: 's', coef: 0.001},
	ms: {unit: 'ms', coef: 1},
	us: {unit: 'µs', coef: 1000},
};

/**
 * Converts a number to a fixed decimal string, removing trailing zeros.
 *
 * @param {number} val - The number to convert.
 * @param {number} prec - Decimal precision.
 * @returns {string} A string representation of the number without trailing zeros.
 */
function toFixedNoTrailingZeros(val, prec) {
	return parseFloat(val.toFixed(prec)).toString();
}

/**
 * Truncates a number to a certain precision without rounding, then removes trailing zeros.
 *
 * @param {number} val - The number to truncate.
 * @param {number} prec - Decimal precision.
 * @returns {string} A truncated, not rounded string representation.
 */
function toFixedNoTrailingZerosTrunc(val, prec) {
	const mult = Math.pow(10, prec);
	return toFixedNoTrailingZeros(Math.trunc(mult * val) / mult, prec);
}

/**
 * Humanizes a duration (in milliseconds) to a human-readable string,
 * choosing appropriate units (ns, µs, ms, s, m, h).
 *
 * @param {number} duration - The duration in milliseconds.
 * @returns {string} Human-readable duration (e.g. "2.5ms", "3s", "1m30s").
 */
function humanizeGenericDuration(duration) {
	if (duration === 0) {
		return '0s';
	}

	if (duration < 0.001) {
		// smaller than a microsecond, print nanoseconds
		return Math.trunc(duration * 1000000) + 'ns';
	}
	if (duration < 1) {
		// smaller than a millisecond, print microseconds
		return toFixedNoTrailingZerosTrunc(duration * 1000, 2) + 'µs';
	}
	if (duration < 1000) {
		// duration is smaller than a second
		return toFixedNoTrailingZerosTrunc(duration, 2) + 'ms';
	}

	let fixedDuration =
		toFixedNoTrailingZerosTrunc(
			(duration % 60000) / 1000,
			duration > 60000 ? 0 : 2,
		) + 's';
	let rem = Math.trunc(duration / 60000);
	if (rem < 1) {
		// less than a minute
		return fixedDuration;
	}
	fixedDuration = (rem % 60) + 'm' + fixedDuration;
	rem = Math.trunc(rem / 60);
	if (rem < 1) {
		// less than an hour
		return fixedDuration;
	}
	return rem + 'h' + fixedDuration;
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
		return (
			(dur * unitMap[timeUnit].coef).toFixed(2) + unitMap[timeUnit].unit
		);
	}

	return humanizeGenericDuration(dur);
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
			];
		case 'gauge':
			return [
				humanizeValue(metric.values.value, metric, timeUnit),
				'min=' + humanizeValue(metric.values.min, metric, timeUnit),
				'max=' + humanizeValue(metric.values.max, metric, timeUnit),
			];
		case 'rate':
			return [
				humanizeValue(metric.values.rate, metric, timeUnit),
				`${metric.values.passes} out of ${metric.values.passes + metric.values.fails}`,
			];
		default:
			return ['[no data]'];
	}
}
