import ssh from 'k6/x/ssh';

export default function () {
  ssh.connect({
    username: 'sshuser',
    host: 'localhost',
    port: 2222,
    rsa_key: 'examples/example_rsa' // "~/.ssh/id_rsa" by default
  })
  console.log(ssh.run('pwd'))
  console.log(ssh.run('ls -la'))
}
