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

The creation takes about 3 minutes. You can view the progress of the stack creation at https://eu-west-1.console.aws.amazon.com/cloudformation/home?region=eu-west-1#/stacks?filter=active

The public and private IPs for the created servers are returned as Output data from the stack creation. In the above UI you can click the "Outputs" tab to see all output variables from the stack creation. You can also use the CLI:

```
aws cloudformation describe-stacks --stack-name "SpeedboatTest1"

{
    "Stacks": [
        {
            "StackId": "arn:aws:cloudformation:eu-west-1:841028731407:stack/SpeedboatTest1/b898d590-fb1f-11e5-80b3-50faeb53b42a", 
            "Tags": [], 
            "Outputs": [
                {
                    "Description": "Public IP of target machine", 
                    "OutputKey": "TargetServerPublicIP", 
                    "OutputValue": "52.50.49.4"
                }, 
                {
                    "Description": "Private IP of target machine", 
                    "OutputKey": "TargetServerPrivateIP", 
                    "OutputValue": "10.0.0.22"
                }, 
                {
                    "Description": "Public IP of load generator machine", 
                    "OutputKey": "LoadgenServerPublicIP", 
                    "OutputValue": "52.18.137.216"
                }, 
                {
                    "Description": "Private IP of load generator machine", 
                    "OutputKey": "LoadgenServerPrivateIP", 
                    "OutputValue": "10.0.0.109"
                }
            ], 
            "CreationTime": "2016-04-05T11:15:36.441Z", 
            "StackName": "SpeedboatTest1", 
            "NotificationARNs": [], 
            "StackStatus": "CREATE_COMPLETE", 
            "DisableRollback": false
        }
    ]
}

```


Note that we can only have a certain number of stacks at any one time so even if you e.g. shut down servers or whatnot it is important to also delete the stacks. The simplest thing is to always delete a stack when shutting things down.


