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

async function load() {
  let {
    onCLS, onFID, onLCP, onFCP, onINP, onTTFB
  } = await import('https://unpkg.com/web-vitals@3?module');

  onCLS(print);
  onFID(print);
  onLCP(print);

  onFCP(print);
  onINP(print);
  onTTFB(print);
}

load();
