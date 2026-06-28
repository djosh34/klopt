NEVER EVER CHANGE .golangci.yml for ANY REASON!
EVEN IF YOU THINK THIS OR THAT IS 'UNSUPPORTED' (YOU ARE WRONG, DONT FUCKING CHANGE IT)

you are not allowed to create stuff like stringPtr and boolPtr, instead, because of go1.26+ you MUST use new("string") instead
this WORKS, EVEN WHEN THE EXPRESSION IS NOT A TYPE!