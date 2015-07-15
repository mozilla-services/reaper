# AWS Reaper

## About

The Reaper terminates forgotten AWS resources.

Reaper workflow:
1. Find all enabled resources, filter them, then fire events based on config
    1.a. Event types include sending emails, posting events to Datadog (Statsd), tagging resources on AWS, stopping or killing resources, and more
2.b. Reaper uses the `Owner` tag on resources to notify the owner of the resource with options to Ignore (for a time), Whitelist, Terminate, or Stop each resource they own
2. Report statistics about the resources that were found
3. Terminate or Stop abandoned resources after a set amount of time

*Caution* This app is experimental because:
* doesn't have a lot of tests (Read: any), will add when SDK is updated
* it's maintained by an intern

## Building
* checkout repo
* build binary: `godep go build .`
* use binary: `./reaper -config config/default.toml`
* for command line options: `./reaper -help`

## Creating a configuration file
* TODO this will be better documented later
* see `config/default.toml` for documentation
