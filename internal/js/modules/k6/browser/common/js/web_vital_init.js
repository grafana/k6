function print(metric) {
  const m = {
    id: metric.id,
    name: metric.name,
    value: metric.value,
    rating: metric.rating,
    delta: metric.delta,
    numEntries: metric.entries.length,
    navigationType: metric.navigationType,
    url: window.location.href,
  }
  window.k6browserSendWebVitalMetric(JSON.stringify(m))
}

function load() {
  webVitals.onCLS(print, {reportAllChanges: true});
  webVitals.onLCP(print, {reportAllChanges: true});

  webVitals.onFCP(print, {reportAllChanges: true});
  webVitals.onINP(print, {reportAllChanges: true});
  webVitals.onTTFB(print, {reportAllChanges: true});
}

load();
