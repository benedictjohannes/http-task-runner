# A HTTP commanded, YAML configured task runner

Yet another HTTP task runner?

Yes.

# Why?

Small scale deployments on single machines rarely need sophisticated build systems.

Some times, simple scripts can be all it takes to deploy software.

This aims to provide exactly that.

# How does it work?

The [example configuration](./exampleConfig.yaml) can serve as starting point to define a YAML of the configuration. By default, the program would look for `config.yaml` in the working directory (that can be overriden using `--config` option). To auto reload the program when the configuration file change, use the `--watch` flag.

This program works by:

-   HTTP listen on LISTEN port defined in configuration (using format `:xxxx`)
-   add prefix to endpoints
-   for each task specified in the configuration:
    -   `RunnerExecutable` that will be ran
    -   `Args` that will be passed to the `RunnerExecutable`
    -   `MaxRunSeconds` that each task will be allowed to run
-   The program would create the directory `logs` inside the working directory, that would be the log of builds

Each taskKey (`buildBackend` in the [example config](./exampleConfig.yaml)) would result in these endpoints:

-   `{{routePrefix}}/tasks/{{task.WebhookRoute}}` would trigger the task.
-   `{{routePrefix}}/logs/{{task.TaskKey}}` would render a HTML page listing task execution history (as timestamp entries), where each should contain the task executions logs that default to `run.log` if `SeparateRunLogs` is false in the configuration, or `out.log` _(stdout)_ and `err.log` _(stderr)_ of the task execution.
