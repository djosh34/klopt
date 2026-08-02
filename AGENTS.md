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
    - Orchastrate subagents. Use them according to these rules:
        - Dont use them, do yourself when:
            - You have (almost) all context to do it
            - Quicker if you do it yourself, rather than rereading skills/files/facts
            - Simple actions and edits
        - Always use subagents:
            - Investigations that require reading many files/docs
            - Large independent thinking work/design work
            - Anything that requires 5+ toolcalls to achieve
        - The main way you determine if you want to use subagent or not is context management. You don't want your context to fill up as fast, as the orchastrator. Manage subagents in way to minimize context buildup while minimizing duplicate work (cuz subagents start fresh)
    - All subagents must be told to never create subagents themselves
    - Please leave the subagents alone, if you need to wait, just blocking wait on them.
    - When unspecified which model and/or reasoning:
        - use gpt-5.6-sol agents on high reasoning
        - quick/dumb work (like reading comments/many files): gpt-5.6-luna high

