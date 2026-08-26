import ssh from 'k6/x/ssh';

export default function () {
  ssh.connect({
    username: 'sshuser',
    host: 'localhost',
    port: 2222,
    rsa_key: 'examples/example_passphrase_rsa',
    passphrase: 'example_passphrase'
  })
  console.log(ssh.run('pwd'))
  console.log(ssh.run('ls -la'))
}
