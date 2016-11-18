export let CounterType = 1;
export let GaugeType = 2;
export let TrendType = 3;

export class Metric {
	constructor(t, name) {
		if (!__initapi__) {
			throw new Error("Metrics can only be created during the init phase");
		}
		this._impl = __initapi__.NewMetric(t, name);
	}
}

export class Counter extends Metric {
	constructor(name) {
		super(CounterType, name);
	}
}

export class Gauge extends Metric {
	constructor(name) {
		super(GaugeType, name);
	}
}

export class Trend extends Metric {
	constructor(name) {
		super(TrendType, name);
	}
}

export default {
	CounterType: CounterType,
	GaugeType: GaugeType,
	TrendType: TrendType,
	Metric: Metric,
	Counter: Counter,
	Gauge: Gauge,
	Trend: Trend,
}
