import grpc from 'k6/net/grpc';
import { check } from 'k6';

const client = new grpc.Client();
client.load(['definitions'], 'hello.proto');

export default () => {
  client.connect('localhost:10000', {
    plaintext: true
  });

  const data = { greeting: 'Bert' };
  const response = client.invoke('hello.HelloService/SayHello', data);

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  // If there's an error with details, log them
  if (response.error && response.error.details) {
    console.log('Error details:', JSON.stringify(response.error.details, null, 2));
    
    response.error.details.forEach((detail, index) => {
      console.log(`Detail ${index} type:`, detail['@type']);
      
      // Example: Check for ErrorInfo
      if (detail['@type'] && detail['@type'].includes('ErrorInfo')) {
        console.log('  Reason:', detail.reason);
        console.log('  Domain:', detail.domain);
        console.log('  Metadata:', JSON.stringify(detail.metadata));
      }
      
      // Example: Check for RetryInfo
      if (detail['@type'] && detail['@type'].includes('RetryInfo')) {
        console.log('  Retry delay:', detail.retryDelay);
      }
      
      // Example: Check for BadRequest
      if (detail['@type'] && detail['@type'].includes('BadRequest')) {
        console.log('  Field violations:');
        detail.fieldViolations.forEach((violation) => {
          console.log(`    Field: ${violation.field}, Description: ${violation.description}`);
        });
      }
    });
  }

  client.close();
};
