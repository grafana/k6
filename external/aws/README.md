AWS Test Setup
==============

To set up a lab environment on Amazon AWS (on the `eu-west-1` region):

1. **Install Terraform**  
   
   Official instructions are provided at [terraform.io](https://terraform.io/).

2. **Prepare credentials**
   
   Create a file called `terraform.tfvars` in this directory, containing:
   
   ```
   access_key = "YOUR_ACCESS_KEY"
   secret_key = "YOUR_SECRET_KEY"
   ```

3. **Prepare an access key**
   
   Generate a key called `k6-test` on the IAM page for your user, and save it as `k6-test.pem` in this directory.
   
   If you already have a different key you'd like to use, you can add the following to `terraform.tfvars` and save it as `YOUR_KEY_NAME.pem` instead:
   
   ```
   key_name = "YOUR_KEY_NAME"
   ```

4. **Let Terraform do the thing**
   
   ```
   terraform apply
   ```
   
   Now just sit back and wait for a few minutes. Seriously, this takes a while.
   Take note of the `loadgen_ip`, `influx_ip` and `web_ip` printed at the end.

5. **Do the other thing**
   
   ```
   ssh -i k6-test.pem ubuntu@LOADGEN_IP
   
   # on loadgen
   sudo salt '*' state.highstate
   ```
