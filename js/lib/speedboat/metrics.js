export let CounterType = 1;
export let GaugeType = 2;
export let TrendType = 3;

export class Metric {
	constructor(t, name, isTime=false) {
		if (!__initapi__) {
			throw new Error("Metrics can only be created during the init phase");
		}
		this._impl = __initapi__.NewMetric(t, name, !!isTime);
	}

	add(v, tags={}) {
		if (!__jsapi__) {
			throw new Error("Metric.add() needs VU context")
		}
		__jsapi__.MetricAdd(this._impl, v*1.0, tags);
		return v;
	}
}

export class Counter extends Metric {
	constructor(name, isTime=false) {
		super(CounterType, name, isTime);
	}
}

export class Gauge extends Metric {
	constructor(name, isTime=false) {
		super(GaugeType, name, isTime);
	}
}

export class Trend extends Metric {
	constructor(name, isTime=false) {
		super(TrendType, name, isTime);
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
