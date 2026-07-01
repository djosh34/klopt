## Goal

Design additive test cases for decode/validate behavior for:

```yaml
/nullable-object-keys-additional-properties-false
```

This is documentation only for now, not implementation.

## Scope

List valid and invalid cases for each layer:

```text
requestBody
schema object
each individual property schema
```

Each group starts with the relevant YAML snippet.

## Main rule

Avoid combination explosion. Do not enumerate every object plus every property value combination.

```text
object cases cover object concerns only
property cases cover property values only
```

Object cases should use placeholders:

```text
<valid requiredNullableString>
<valid requiredNotNullableString>
<valid optionalNullableString>
<valid optionalNotNullableString>
```

Object cases focus on required keys, optional key presence, top-level nullable behavior, object-vs-non-object shape, and `additionalProperties: false`.

Property cases should show only the raw JSON value being tested:

```json
"some string"
```

```json
null
```

```json
123
```

## Markdown shape

```text
## for type/layer groups
### Valid and ### Invalid
#### for individual case names
--- between cases
```

Keep the case file focused on cases. Do not include generated Go model names or details already readable from `models.go`.
