# AWS Reaper

## About

The reaper terminates forgotten EC2 instances. It works like this:

1. Qualifies instances that have been running for a specific amount of time
2. Notifies the owner of the instance via email with option to delay termination or terminate immediately. Owners are identified by an `Owner` tag with an email address
3. Terminates the instances after the configured amount of time

*Caution* This app is experimental because:

* relies on an experimental version of the [AWS SDK](https://github.com/awslabs/aws-sdk-go). This is to be updated when the SDK reaches a more stable state
* doesn't have a lot of tests, will add when SDK is updated
* Run it at your own risk

## Building

* checkout repo
* build binary: `godep go build main.go`
* use binary: `./reaper -conf config/default.toml`
* for command line options: `./reaper -help`

## Creating a configuration file

* TODO this will be better documented later
* see `config/default.toml` for documentation