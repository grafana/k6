Cloudformation config for test environment
------------------------------------------

The environment will be created in the eu-west-1 AWS data center, and will consist of a VPC containing two m4.xlarge servers where one is meant to be the load generator machine, and the other the sink/target machine.

### Get your AWS API key

- Go to https://console.aws.amazon.com/iam/home?#home
- Find your user and open its security credentials pane
- Create and download your access key, also copy the access key ID

### Install and configure aws command line tools

```
pip install awscli
aws configure
```

Now you get to enter your access key details.

### Creating the stack

```
aws cloudformation create-stack --stack-name "SpeedboatTest1" --template-body 'file:///Users/ragnarlonn/Downloads/speedboat-test1.json'
```

You can view the progress of the stack creation at https://eu-west-1.console.aws.amazon.com/cloudformation/home?region=eu-west-1#/stacks?stackId=arn:aws:cloudformation:eu-west-1:841028731407:stack%2FSpeedboatTest1%2Fb898d590-fb1f-11e5-80b3-50faeb53b42a&filter=active

The public and private IPs for the created servers are returned as Output data from the stack creation. In the above UI you can click the "Outputs" tab to see all output variables from the stack creation.

