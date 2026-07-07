NEVER EVER CHANGE .golangci.yml for ANY REASON!
EVEN IF YOU THINK THIS OR THAT IS 'UNSUPPORTED' (YOU ARE WRONG, DONT FUCKING CHANGE IT)

you are not allowed to create stuff like stringPtr and boolPtr, instead, because of go1.26+ you MUST use new("string") instead
this WORKS, EVEN WHEN THE EXPRESSION IS NOT A TYPE!

Keep it stupid simple
Never 'prepare' for future stuff
Do not create extra fields/functions without reason that you need it

Never ignore errors.


Always use resources/OpenAPI%20Specification%20v3.0.3.mhtml as sole reference on openapi 3.0.3