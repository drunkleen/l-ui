# Agent Usage Policy

The user should not need to manually choose agents.

When the user describes a bug, feature, refactor, docs task, or architecture task, OpenCode must route automatically.

Default routing:

- unclear task → `@planner`
- architecture/API/DB/protocol task → `@architect`
- hub backend task → `@backend`
- frontend/UI task → `@frontend`
- remote VPS/Xray/node task → `@agent`
- auth/token/secret/user-data task → `@security`
- docs task → `@docs`
- after code changes → `@reviewer`

## Default Pipelines

Bug:

```txt
@planner → implementation agent → @security if needed → @reviewer
````

Feature:

```txt
@planner → @architect if needed → implementation agent(s) → @security if needed → @reviewer → @docs if needed
```

Docs:

```txt
@docs → @reviewer
```

UI:

```txt
@planner → @frontend → @reviewer
```

Hub-Agent:

```txt
@planner → @architect → @agent + @backend → @security → @reviewer
```

## Important Rule

Do not wait for the user to say `@backend`, `@frontend`, or `@security`.

The user may write naturally:

```txt
users cannot sync subscriptions
```

OpenCode should decide the route itself.
