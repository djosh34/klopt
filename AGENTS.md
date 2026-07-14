NEVER EVER CHANGE .golangci.yml, unless it is to add to depguard allow list
EVEN IF YOU THINK THIS OR THAT IS 'UNSUPPORTED' (YOU ARE WRONG, DONT FUCKING CHANGE IT)

you are not allowed to create stuff like stringPtr and boolPtr, instead, because of go1.26+ you MUST use new("string") instead
this WORKS, EVEN WHEN THE EXPRESSION IS NOT A TYPE!

Keep it stupid simple
Never 'prepare' for future stuff
Do not create extra fields/functions without reason that you need it

Never ignore errors.

use make lint & fmt, instead of gofump directly

### Please use online references to validate openapi logic/spec, including but not limited to:

Official JSONSchema for openapi 3.0.3: https://spec.openapis.org/oas/3.0/schema/2024-10-18.html 
SchemaObject spec openapi 3.0.3: https://spec.openapis.org/oas/v3.0.3.html#schema-object
JSON Schema dialect that OpenAPI 3.0.3 extends as an extended subset: https://datatracker.ietf.org/doc/html/draft-wright-json-schema-00#section-4.2

### More resources:

data types: https://swagger.io/docs/specification/v3_0/data-models/data-types/
enums: https://swagger.io/docs/specification/v3_0/data-models/enums/
oneOf, allOf, anyOf: https://swagger.io/docs/specification/v3_0/data-models/oneof-anyof-allof-not/
$ref explanation: https://swagger.io/docs/specification/v3_0/using-ref/ 