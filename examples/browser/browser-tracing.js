const ROUTE_PATTERN = /./;

function generateHexString(length) {
  const chars = '0123456789abcdef';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars[Math.floor(Math.random() * chars.length)];
  }
  if (/^0+$/.test(result)) {
    result = result.slice(0, -1) + '1';
  }
  return result;
}

function generateTraceID() {
  return generateHexString(32);
}

function generateSpanID() {
  return generateHexString(16);
}

function shouldSample(rate) {
  if (rate <= 0) return false;
  if (rate >= 1) return true;
  return Math.random() < rate;
}

function buildW3CHeader(traceID, spanID, sampled) {
  const flags = sampled ? '01' : '00';
  return `00-${traceID}-${spanID}-${flags}`;
}

function buildJaegerHeader(traceID, spanID, sampled) {
  const flags = sampled ? '1' : '0';
  return `${traceID}:${spanID}:0:${flags}`;
}

export function instrumentBrowser(page, options = {}) {
  const propagator = options.propagator || 'w3c';
  const samplingRate = options.sampling !== undefined ? options.sampling : 1.0;

  if (propagator !== 'w3c' && propagator !== 'jaeger') {
    throw new Error(`invalid propagator: "${propagator}", must be "w3c" or "jaeger"`);
  }
  if (samplingRate < 0 || samplingRate > 1) {
    throw new Error(`invalid sampling rate: ${samplingRate}, must be between 0.0 and 1.0`);
  }

  const traceID = generateTraceID();
  const sampled = shouldSample(samplingRate);
  const headerName = propagator === 'w3c' ? 'traceparent' : 'uber-trace-id';
  const buildHeader = propagator === 'w3c' ? buildW3CHeader : buildJaegerHeader;

  console.log(`browser tracing enabled: propagator=${propagator} traceID=${traceID} sampled=${sampled}`);

  return page.route(ROUTE_PATTERN, async function (route) {
    try {
      const spanID = generateSpanID();
      const headerValue = buildHeader(traceID, spanID, sampled);
      const existingHeaders = route.request().headers() || {};

      await route.continue({
        headers: {
          ...existingHeaders,
          [headerName]: headerValue,
        },
      });
    } catch (e) {
      console.log(`browser tracing: error continuing request: ${e}`);
    }
  });
}

export function uninstrumentBrowser(page) {
  return page.unrouteAll();
}
