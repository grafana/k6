import ssh from 'k6/x/ssh';

export default function () {
  ssh.connect({
    username: 'sshuser',
    password: 'secret-password',
	host: 'localhost',
	port: 2222
  })
  console.log(ssh.run('pwd'))
  console.log(ssh.run('ls -la'))
}
