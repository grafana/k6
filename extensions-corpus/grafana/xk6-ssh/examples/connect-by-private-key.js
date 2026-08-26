import ssh from 'k6/x/ssh';

// The private key contents are provided in-memory (e.g. fetched from a secrets
// manager such as Vault or k6/secrets) instead of being read from a file on disk.
export default function () {
  ssh.connect({
    username: 'sshuser',
    host: 'localhost',
    port: 2222,
    private_key: __ENV.SSH_PRIVATE_KEY, // PEM contents, NOT a file path
    // passphrase: __ENV.SSH_PASSPHRASE, // optional, if the key is encrypted
  });
  console.log(ssh.run('pwd'));
}