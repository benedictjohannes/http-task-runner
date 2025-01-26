type JSONContentTest = {
    Key: string
    Value: string | number | boolean
}
type TestConditions = {
    Header?: Record<string, any>
    JSONBody?: JSONContentTest[]
}
type Task = {
    RunnerExecutable: string
    Args: string[]
    /** defaults to 60 seconds */
    MaxRunSeconds?: number
    TaskKey: string
    WebhookRoute: string
    Tests: TestConditions
}
export type ConfigSchema = {
    Listen: string
    AppName: string
    RoutePrefix: string
    Tasks: Task[]
}
