import remote from 'k6/x/remotewrite';
import { check } from 'k6';

export const options = {
    vus: 1,
    iterations: 1,
    thresholds: {
        'checks': ['rate==1.0'],
    },
};

export default function () {
    // Check that all exported symbols exist
    check(remote, {
        'Client constructor exists': (r) => typeof r.Client === 'function',
        'Sample constructor exists': (r) => typeof r.Sample === 'function',
        'Timeseries constructor exists': (r) => typeof r.Timeseries === 'function',
        'precompileLabelTemplates exists': (r) => typeof r.precompileLabelTemplates === 'function',
    });

    // Test Client constructor
    const client = new remote.Client({ url: 'http://localhost:9090/api/v1/write' });

    check(client, {
        'Client instance created': (c) => c !== undefined,
        'Client.store method exists': (c) => typeof c.store === 'function',
        'Client.storeGenerated method exists': (c) => typeof c.storeGenerated === 'function',
        'Client.storeFromTemplates method exists': (c) => typeof c.storeFromTemplates === 'function',
        'Client.storeFromPrecompiledTemplates method exists': (c) => typeof c.storeFromPrecompiledTemplates === 'function',
    });

    // Test precompileLabelTemplates
    const template = {
        __name__: 'test_metric_${series_id}',
        series_id: '${series_id}',
    };
    const compiled = remote.precompileLabelTemplates(template);
    check(compiled, {
        'precompileLabelTemplates returns object': (c) => c !== undefined && typeof c === 'object',
    });
}