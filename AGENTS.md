NEVER EVER CHANGE .golangci.yml, unless it is to add to depguard allow list

you are not allowed to create stuff like stringPtr and boolPtr, instead, because of go1.26+ you MUST use new("string") instead

Keep it stupid simple
Never 'prepare' for future stuff
Do not create extra fields/functions without reason that you need it

Never ignore errors.

use make lint & fmt, instead of gofump directly

### Please use online references to validate openapi logic/spec, including but not limited to:

When grilling, always first fact check against the official spec, before asking me to decide.
Always aim to follow the spec, always confirm with me if we want to intentionally deviate from the spec.

Official JSONSchema for openapi 3.0.3: https://spec.openapis.org/oas/3.0/schema/2024-10-18.html 
SchemaObject spec openapi 3.0.3: https://spec.openapis.org/oas/v3.0.3.html#schema-object
JSON Schema dialect that OpenAPI 3.0.3 extends as an extended subset: https://datatracker.ietf.org/doc/html/draft-wright-json-schema-00#section-4.2

When deviating from the spec, we hard and loudly reject (must return error, never silent) during Parse phase and not during Validate phase, unless stated otherwise

you get explicit permission here to use subagents
