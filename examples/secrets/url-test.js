import secrets from 'k6/secrets';

export default async () => {
  console.log('Fetching secret from URL source...');
  const my_secret = await secrets.get('api-key');
  console.log(my_secret == "super-secret-api-key-12345");
};
