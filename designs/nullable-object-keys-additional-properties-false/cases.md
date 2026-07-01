## requestBody

```yaml
requestBody:
  required: true
  content:
    application/json:
      schema:
        type: object
        nullable: true
        required:
          - requiredNullableString
          - requiredNotNullableString
        additionalProperties: false
        properties:
          requiredNullableString:
            type: string
            nullable: true
          requiredNotNullableString:
            type: string
            nullable: false
          optionalNullableString:
            type: string
            nullable: true
          optionalNotNullableString:
            type: string
            nullable: false
```

### Valid

#### body present with declared content type

```text
Content-Type: application/json

<valid schema value>
```

---

### Invalid

#### missing body

```text
<missing request body>
```

---

#### empty body

```text
<empty request body>
```

---

#### malformed json body

```text
{"requiredNullableString":
```

---

#### unsupported content type

```text
Content-Type: text/plain

<valid schema value>
```

---

## object nullable true additionalProperties false

```yaml
schema:
  type: object
  nullable: true
  required:
    - requiredNullableString
    - requiredNotNullableString
  additionalProperties: false
  properties:
    requiredNullableString:
      type: string
      nullable: true
    requiredNotNullableString:
      type: string
      nullable: false
    optionalNullableString:
      type: string
      nullable: true
    optionalNotNullableString:
      type: string
      nullable: false
```

### Valid

#### null object

```json
null
```

---

#### required properties present and optional properties omitted

```text
{
  "requiredNullableString": <valid requiredNullableString>,
  "requiredNotNullableString": <valid requiredNotNullableString>
}
```

---

#### optional nullable property present

```text
{
  "requiredNullableString": <valid requiredNullableString>,
  "requiredNotNullableString": <valid requiredNotNullableString>,
  "optionalNullableString": <valid optionalNullableString>
}
```

---

#### optional not nullable property present

```text
{
  "requiredNullableString": <valid requiredNullableString>,
  "requiredNotNullableString": <valid requiredNotNullableString>,
  "optionalNotNullableString": <valid optionalNotNullableString>
}
```

---

### Invalid

#### string instead of object

```json
"not-object"
```

---

#### number instead of object

```json
123
```

---

#### boolean instead of object

```json
true
```

---

#### array instead of object

```json
[]
```

---

#### both required properties omitted

```text
{}
```

---

#### required nullable property omitted

```text
{
  "requiredNotNullableString": <valid requiredNotNullableString>
}
```

---

#### required not nullable property omitted

```text
{
  "requiredNullableString": <valid requiredNullableString>
}
```

---

#### additional property present

```text
{
  "requiredNullableString": <valid requiredNullableString>,
  "requiredNotNullableString": <valid requiredNotNullableString>,
  "extra": <any json value>
}
```

---

## requiredNullableString value

```yaml
required:
  - requiredNullableString
properties:
  requiredNullableString:
    type: string
    nullable: true
```

### Valid

#### string

```json
"required-nullable"
```

---

#### empty string

```json
""
```

---

#### null

```json
null
```

---

### Invalid

#### number

```json
123
```

---

#### boolean

```json
true
```

---

#### object

```json
{}
```

---

#### array

```json
[]
```

---

## requiredNotNullableString value

```yaml
required:
  - requiredNotNullableString
properties:
  requiredNotNullableString:
    type: string
    nullable: false
```

### Valid

#### string

```json
"required-not-nullable"
```

---

#### empty string

```json
""
```

---

### Invalid

#### null

```json
null
```

---

#### number

```json
123
```

---

#### boolean

```json
false
```

---

#### object

```json
{}
```

---

#### array

```json
[]
```

---

## optionalNullableString value

```yaml
properties:
  optionalNullableString:
    type: string
    nullable: true
```

### Valid

#### string

```json
"optional-nullable"
```

---

#### empty string

```json
""
```

---

#### null

```json
null
```

---

### Invalid

#### number

```json
123
```

---

#### boolean

```json
false
```

---

#### object

```json
{}
```

---

#### array

```json
[]
```

---

## optionalNotNullableString value

```yaml
properties:
  optionalNotNullableString:
    type: string
    nullable: false
```

### Valid

#### string

```json
"optional-not-nullable"
```

---

#### empty string

```json
""
```

---

### Invalid

#### null

```json
null
```

---

#### number

```json
123
```

---

#### boolean

```json
true
```

---

#### object

```json
{}
```

---

#### array

```json
[]
```
