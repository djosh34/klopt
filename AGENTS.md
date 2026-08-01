NEVER EVER CHANGE .golangci.yml, unless it is to add to depguard allow list

you are not allowed to create stuff like stringPtr and boolPtr, instead, because of go1.26+ you MUST use new("string") instead

Keep it stupid simple
Rather boldly refactor than to create bad and spaghetti code
This is greenfield project with 0 users, do not incorporate backwards compat

Never 'prepare' for future stuff
Do not create extra fields/functions without reason that you need it

Never ignore errors.

use make lint & fmt, instead of gofump directly

ALWAYS crosscheck with 3.0.x openapi spec, before asking me anything. you MUST have checked the official resources first

When deviating from the openapi spec, we hard and loudly reject (must return error, never silent) during Parse phase and not during Validate phase, unless stated otherwise

When your prompt DOES contain smth like 'Do not create subagents':
    - Never create new agent (no create_agent, no subagent, no codex, no whatever)
    - If you feel like you need to create agent, that is wrong!

Follow these rules ONLY when your prompt does NOT contain smth like 'Do not create subagents':
    - Always orchastrate EVERYTHING (and i mean EVERYTHING) to subagents. Never do anything yourself.
    - All subagents must be told to never create subagents themselves
    - When subagents are done, always close them, never reuse them. Always create new ones
    - Please leave the subagents alone, if you need to wait, just blocking wait on them.
    - When unspecified which model and/or reasoning:
        - use gpt-5.6-sol agents on high reasoning

