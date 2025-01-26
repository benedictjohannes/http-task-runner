type JSONContentTest = {
    Key: string
    Value: string | number | boolean
}
type TestConditions = {
    Header?: Record<string, any>
    JSONBody?: JSONContentTest[]
}
type Task = {
    /** Optional, unique, must be `0-9a-zA-Z-_.`, used for logs entries. When not set, the task logging is disabled and does not appear in logs list HTML. */
    TaskKey?: string 
    /** Optional, must be `0-9a-zA-Z-_.`, registers the route `{{RoutePrefix}}/tasks/{{Route}}`. When not set, the task webhook is disabled. */
    WebhookRoute?: string 
    RunnerExecutable: string
    Args: string[]
    /** defaults to 60 seconds */
    MaxRunSeconds?: number
    Tests: TestConditions
}
export type ConfigSchema = {
    Listen: string
    AppName: string
    RoutePrefix: string
    Tasks: Task[]
}
