# Filters

## About Filters

Filters are implemented per resource. To use a filter for a resource, add it to a filtergroup in that resource's section of your configuration file.

For example:

```
[AutoScalingGroups]
    Enabled = false

    [AutoScalingGroups.FilterGroups]
        [AutoScalingGroups.FilterGroups.ExampleGroup]
            [AutoScalingGroups.FilterGroups.ExampleGroup.SizeGreaterThan1]
                function = "SizeGreaterThanOrEqualTo"
                arguments = ["1"]
```

Here, we have a filter called `SizeGreaterThan1`, which calls the function `SizeGreaterThanOrEqualTo` with the arguments `["1"]`. `SizeGreaterThan1` is in a filtergroup called `ExampleGroup`.

_All filters take an array of arguments. Many filters take a single argument. All arguments are quoted._

## Filter Types:

#### Boolean Filters:

These filters take a single argument, a string that is parsed to a boolean value that is compared with the resource's value for the specified function. This boolean value can be anything parseable by Go's strconv.ParseBool. See: http://golang.org/pkg/strconv/#ParseBool.

#### String Filters:

These filters take a single argument (except where noted), a string that is compared with the resource's value for the specified function.

#### Time Filters:

These filters take a single argument, a string that is parsed to a time in RFC3339 format (parseable by http://godoc.org/time#Parse) or a duration (see: http://godoc.org/time#ParseDuration).

#### Integer Filters:

These filters take a single argument, a string that is parsed to an int64 and compared with the resource's value for the specified function.

## Shared Filters (All Resource Types)

#### Boolean Filters:

- IsDependency
    + Whether the resource is a dependency for another resource (a bit abstract)
    + Currently, a resource is a dependency if any of the following are satisfied:
        * the resource is in the list of resources of a Cloudformation
        * the resource is in an AutoScalingGroup
        * the resource is a SecurityGroup used by an Instance

#### String Filters:

- Tagged
    + True if the resource has a tag equal to the input string
- NotTagged
    + True if the resource does not have a tag equal to the input string
- Tag (takes two arguments)
    + argument 1: the key of a tag
    + argument 2: the value of that tag
    + True if the resource has a tag equal to the first argument with a value equal to the second
- TagNotEqual (takes two arguments)
    + argument 1: the key of a tag
    + argument 2: the value of that tag
    + True if the resource does not have a tag equal to the first argument with a value equal to the second
- Region (takes any number of arguments)
    + True if the resource's region matches the input string
- NotRegion
    + True if the resource's region does not match the input string
- ReaperState:
    + True if the resource's ReaperState is equal to the input string
    + One of:
        * FirstState
        * SecondState
        * ThirdState
        * FinalState
        * IgnoreState
- NotReaperState:
    + True if the resource's ReaperState is not equal to the input string
- NameContains:
    + True if the resource's name contains the input string
- NotNameContains:
    + True if the resource's name does not contain the input string
- Named:
    + True if the resource's name is equal to the input string
- NotNamed:
    + True if the resource's name is not equal to the input string

## Instance Only Filters:

#### Boolean Filters:

- InCloudformation
    + Whether the Instance is in a Cloudformation (directly)
- HasPublicIPAddress
    + True if the Instance has a public IP address
- AutoScaled
    + True if the Instance is in an AutoScalingGroup

#### String Filters:

- InstanceType
    + True if the InstanceType of the Instance matches the input string
- State
    + True if the Instance's State matches the input string
    + One of:
        * pending
        * running
        * shutting-down
        * terminated
        * stopping
        * stopped
- PublicIPAddress
    + True if the public IP address of the Instance matches the input string

#### Time Filters:

- LaunchTimeBefore
    + True if the Instance's LaunchTime is before time input time
- LaunchTimeAfter
    + True if the Instance's LaunchTime is after time input time
- LaunchTimeInTheLast
    + True if the Instance's LaunchTime is within the input duration
- LaunchTimeNotInTheLast
    + True if the Instance's LaunchTime is not within the input duration


## AutoScalingGroup Only Filters

#### Time Filters:

- InCloudformation
    + Whether the AutoScalingGroup is in a Cloudformation (directly)
- CreatedTimeInTheLast
    + True if the AutoScalingGroup's CreatedTime is within the input duration
- CreatedTimeNotInTheLast
    + True if the AutoScalingGroup's CreatedTime is not within the input duration

#### Integer Filters:

- SizeGreaterThan
    + True if the AutoScalingGroup's DesiredCapacity is greater than the input size
- SizeLessThan
    + True if the AutoScalingGroup's DesiredCapacity is less than the input size
- SizeEqualTo
    + True if the AutoScalingGroup's DesiredCapacity is equal to the input size
- SizeLessThanOrEqualTo
    + True if the AutoScalingGroup's DesiredCapacity is less than or equal to the input size
- SizeGreaterThanOrEqualTo
    + True if the AutoScalingGroup's DesiredCapacity is greater than or equal to the input size

## Cloudformation Only Filters

#### String Filters:

- Status
    + True if the Status of the Cloudformation matches the input string
        * One of:
            - CREATE_COMPLETE
            - CREATE_IN_PROGRESS
            - CREATE_FAILED
            - DELETE_COMPLETE
            - DELETE_FAILED
            - DELETE_IN_PROGRESS
            - ROLLBACK_COMPLETE
            - ROLLBACK_FAILED
            - ROLLBACK_IN_PROGRESS
            - UPDATE_COMPLETE
            - UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
            - UPDATE_IN_PROGRESS
            - UPDATE_ROLLBACK_COMPLETE
            - UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
            - UPDATE_ROLLBACK_FAILED
            - UPDATE_ROLLBACK_IN_PROGRESS
- NotStatus
    + True if the Status of the Cloudformation does not match the input string

#### Time Filters:

- CreatedTimeInTheLast
    + True if the Cloudformation's CreatedTime is within the input duration
- CreatedTimeNotInTheLast
    + True if the Cloudformation's CreatedTime is not within the input duration
