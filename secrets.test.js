import http from 'k6/http';
import secrets from 'k6/secrets';

export default async () => {
  const my_secret = await secrets.get('cool'); // Retrieves secret by identifier
  console.log(my_secret);
  const response = await http.asyncRequest('GET', 'https://httpbin.org/get', null, {
    headers: {
      'Custom-Authentication': `Bearer ${await secrets.get('else')}`,
    },
  });
  console.log(response.body);
};
