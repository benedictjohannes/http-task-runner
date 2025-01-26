type JSONContentTest = {
    Key: string
    Value: string | number | boolean
}
type TestConditions = {
    /** Optional, strict string equality test for each entries in the Header map */
    Header?: Record<string, any>
    /** Optional, should always begin each property using $ */
    JSONBody?: JSONContentTest[]
}
type Task = {
    /** Optional, unique, must be `0-9a-zA-Z-_.`, used for logs entries. When not set, the task logging is disabled and does not appear in logs list HTML. */
    TaskKey?: string
    /** Optional, must be `0-9a-zA-Z-_.`, registers the route `{{RoutePrefix}}/tasks/{{Route}}`. When not set, the task webhook is disabled. */
    WebhookRoute?: string
    /** Path to the task's executor executable */
    RunnerExecutable: string
    /** Arguments that will be passed to the task executor */
    Args?: string[]
    /** Optional, defaults to 60 seconds */
    MaxRunSeconds?: number
    /** Optional, must pass all criteria for the task to be run */
    Tests?: TestConditions
}
export type ConfigSchema = {
    Listen: string
    AppName: string
    RoutePrefix: string
    Tasks: Task[]
}
