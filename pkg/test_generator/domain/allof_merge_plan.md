# allOf merge plan

Spec basis checked against `resources/OpenAPI Specification v3.0.3.mhtml` only:

- `allOf` validates every child schema independently. A merged domain is therefore the intersection of constraints.
- `type` is a single string in OAS 3.0.3. Type arrays are not supported.
- `nullable: true` allows `null` only where the schema says so. For allOf merging, nullability is an intersection, so it merges with AND.
- `additionalProperties` defaults to `true`.

Pragmatic implementation rules:

- Hard requirement: keep merging stupid simple for as long as that still satisfies these requirements.
- Do not add helper functions unless there is an immediate requirement. In particular, implement each domain merge directly in its respective domain file where `NOT IMPLEMENTED` exists now.
- Do exact intersections only. Do not run a JSON Schema solver.
- Do not validate enum values against patterns, formats, length, items, min/max, properties, or additionalProperties during merge.
- Do not filter enum values by any non-enum field during merge.
- Do not normalize enum values. For numbers, enum value `1` is not the same enum value as `1.0`.
- A `null` enum member is represented as raw JSON bytes equal to `[]byte("null")`; do not convert it into the `Nullable` field.
- Typed enum fields must still be able to carry a raw `null` enum member somehow, because `[]string`, `[]Number`, and `[]bool` cannot store it directly.
- Do not add extra satisfiability checks that the generator will naturally handle by producing no cases.
- Error when the merge is unrepresentable, types conflict, enum exact-set intersection is empty, or property/additional-property schemas cannot be merged.
- All enum fields in every domain must merge the same way: if both sides have enum arrays, take the intersection of the two arrays; if only one side has an enum array, keep that array as-is.
- Reuse the same enum-array intersection function for all enum merging so behavior stays identical across string, number, bool, array, and object domains.
- Merges should be transactional: if an error is returned, do not mutate the receiver.
- Keep deterministic output ordering. For exact-set intersections, preserve the left side order of values that also exist on the right.

## AllOfDomain

- `Domains []types.Domain`
  - Preserve original child domains in schema order for hashing/debugging.
  - Nested allOf should be merged as all-with-all; semantically it is still one conjunction.

- `MergedDomain types.Domain`
  - First child becomes the initial merged domain.
  - Each next child is merged into the accumulated domain using `AllOfMerge`.
  - On any child merge error, return the error and keep the original `AllOfDomain` unchanged.

- nullable on an allOf schema
  - allOf-level nullable must be included in the merge plan.
  - When nullable is present on the allOf wrapper and nullable exists on merged child domains, combine with AND.
  - Absent allOf-level nullable should not accidentally narrow child nullability just because Go bool defaults to false; track presence if needed.

- `AllOfMerge(other)` behavior
  - `nil` receiver or `nil` other: error.
  - `AllOfDomain + AllOfDomain`: merge every child of `other` into this domain in order.
  - `AllOfDomain + non-AllOf`: append `other` to `Domains` and intersect it into `MergedDomain`.
  - `non-AllOf + AllOfDomain`: merge the non-AllOf domain with each allOf child.

- parse behavior
  - allOf items may be any supported Schema Object domain: object, array, string, number/integer, boolean, or another allOf.
  - Sibling schema constraints next to `allOf` should be merged too when supported.
  - `$ref` handling is already filtered/resolved elsewhere; do not add special merge logic for it here.

## StringDomain

- `Nullable bool`
  - Merge with AND.

- `Enum []string`
  - If both sides have enum: exact string/raw-null set intersection.
  - If one side has enum: keep that enum as-is.
  - Raw `null` enum members compare by raw bytes equal to `[]byte("null")`.
  - If an exact enum intersection is empty: error and ask the user to add an enum value that satisfies both schemas.
  - Do not compare enum values to pattern, format, examples, minLength, or maxLength.

- `Pattern types.Pattern`
  - Concatenate left patterns then right patterns.
  - This is for debugging/generation metadata only.
  - Do not parse regexes and do not check whether patterns are mutually satisfiable.

- `Format types.Format`
  - Concatenate left formats then right formats.
  - This is for debugging/generation metadata only.
  - Do not interpret formats and do not check whether formats are mutually satisfiable.

- `XValidExamples []string`
  - If both sides have valid examples: exact string set intersection.
  - If one side has valid examples: keep that set as-is.
  - Empty valid-example intersection without an enum is allowed; it just means no valid examples can be generated from that set.
  - If enum and valid examples both exist after merge: exact string set intersection between them too, because for generation they are effectively the same positive-case set.
  - If the enum/valid-example intersection is empty: error and ask the user to add a value that satisfies both positive sets.
  - Do not check examples against pattern, format, minLength, or maxLength.

- `XInvalidExamples []string`
  - Always union left and right invalid examples.
  - Do not remove values because they appear in valid examples or enum.

- `MinLength int`
  - Keep the larger value.
  - Do not do extra no-cases checks beyond storing the stricter value.

- `MaxLength *int`
  - Keep the smaller non-nil value. `nil` means unbounded.
  - Do not do extra no-cases checks beyond storing the stricter value.

## NumberDomain

- `Type string`
  - `number + number` => `number`.
  - `integer + integer` => `integer`.
  - `number + integer` or `integer + number` => `integer`.
  - Any other type combination: error.

- `Nullable bool`
  - Merge with AND.

- `Enum []Number`
  - If both sides have enum: exact lexeme/raw-null set intersection.
  - If one side has enum: keep that enum as-is.
  - Do not normalize enum values. `1`, `1.0`, and `1e0` are different enum values.
  - Raw `null` enum members compare by raw bytes equal to `[]byte("null")`.
  - If an exact enum intersection is empty: error and ask the user to add an enum value that satisfies both schemas.
  - Do not filter enum values by type, minimum, maximum, exclusivity, multipleOf, or format.

- `Minimum *Number` and `ExclusiveMinimum bool`
  - Parse numbers with `math/big` for comparison.
  - Keep the stricter/larger lower bound.
  - If both lower bounds compare equal, `ExclusiveMinimum` is `left.ExclusiveMinimum || right.ExclusiveMinimum`.
  - Do not normalize the stored number lexeme.
  - Do not do extra range-satisfiability checks.

- `Maximum *Number` and `ExclusiveMaximum bool`
  - Parse numbers with `math/big` for comparison.
  - Keep the stricter/smaller upper bound.
  - If both upper bounds compare equal, `ExclusiveMaximum` is `left.ExclusiveMaximum || right.ExclusiveMaximum`.
  - Do not normalize the stored number lexeme.
  - Do not do extra range-satisfiability checks.

- `MultipleOf *Number`
  - If only one side has it: keep it.
  - If both sides have it: use `math/big` rational arithmetic to merge the constraints into one `multipleOf` value that satisfies both.
  - Do not normalize unrelated number fields while doing this.
  - Do not check the merged `multipleOf` against minimum/maximum range satisfiability.

- `Format *string`
  - `nil` + set => keep set.
  - Same set value + same set value => keep it.
  - Different set values => error, because `NumberDomain` can store only one format.
  - If the merged type is `integer`, `float`/`double` format is unrepresentable and should error.

## BoolDomain

- `Nullable bool`
  - Merge with AND.

- `Enum []bool`
  - If both sides have enum: exact bool/raw-null set intersection.
  - If one side has enum: keep that enum as-is.
  - Raw `null` enum members compare by raw bytes equal to `[]byte("null")`.
  - If an exact enum intersection is empty: error and ask the user to add an enum value that satisfies both schemas.

## ArrayDomain

- `Nullable bool`
  - Merge with AND.

- `Enum []types.Domain`
  - Enum values are raw JSON values wrapped as `EnumDomain`.
  - If both sides have enum: exact raw-byte set intersection.
  - If one side has enum: keep that enum as-is.
  - `null` enum values are allowed as raw bytes equal to `[]byte("null")`.
  - Do not filter enum values by items, minItems, or maxItems.
  - If an exact enum intersection is empty: error and ask the user to add an enum value that satisfies both schemas.

- `Items types.Domain`
  - `nil` means unconstrained `items: {}`.
  - `nil + domain` => keep domain.
  - `domain + nil` => keep domain.
  - `domain + domain` => merge using `AllOfMerge`; errors propagate.

- `MinItems int`
  - Keep the larger value.
  - Do not check whether it conflicts with maxItems.

- `MaxItems *int`
  - Keep the smaller non-nil value. `nil` means unbounded.
  - Do not do extra no-cases checks.

## ObjectDomain

- `Nullable bool`
  - Merge with AND.

- `Enum []types.Domain`
  - Enum values are raw JSON values wrapped as `EnumDomain`.
  - If both sides have enum: exact raw-byte set intersection.
  - If one side has enum: keep that enum as-is.
  - `null` enum values are allowed as raw bytes equal to `[]byte("null")`.
  - Do not filter enum values by properties, required, additionalProperties, minProperties, or maxProperties.
  - If an exact enum intersection is empty: error and ask the user to add an enum value that satisfies both schemas.

- `Properties []types.Domain`
  - Every entry must be `*Property`; otherwise error.
  - Merge by property key.
  - Same key:
    - `Required` is OR.
    - If both have `Domain`: merge those domains with `AllOfMerge`.
    - If one side has `Domain` and the other has nil/unconstrained: keep the concrete domain.
    - If both domains are nil: keep nil.
  - New key from only one side:
    - If the other object has `additionalProperties: true`: keep it.
    - If the other object has `additionalProperties` as a schema: merge this property domain with that additional-property schema.
    - If the other object has `additionalProperties: false` and does not declare this key:
      - required property => error, because one schema requires what the other forbids.
      - optional property => drop it from the merged properties; final `additionalProperties: false` forbids it.
  - Sort final properties by key.

- `Property.Required`
  - Required flags merge with OR.
  - Do not check required property count against maxProperties.

- `AdditionalPropertyKind` and `AdditionalPropertyDomain`
  - `AdditionalFalse` dominates unknown keys.
  - If all are true/default: final is `AdditionalTrue`.
  - If one side has `AdditionalSchema` and the other is true/default: keep the schema.
  - If both sides have `AdditionalSchema`: merge their domains with `AllOfMerge`; errors propagate.
  - If any side is `AdditionalFalse`, final unknown additional properties are false, but still apply the rules above for explicit properties introduced by the other side.
  - `AdditionalSchema` with nil domain is invalid and must error.

- `MinProps int`
  - Keep the larger value.
  - Do not check whether enough properties can exist.

- `MaxProps *int`
  - Keep the smaller non-nil value. `nil` means unbounded.
  - Do not check property counts against it.

## EnumDomain

- `RawMessage *json.RawMessage`
  - Represents exactly one raw JSON enum value, including `null`.
  - `nil` receiver: error.
  - Do not generally merge enum domains as schemas.
  - Use `EnumDomain` only as a raw-byte value for exact enum set intersection.
  - `EnumDomain` compared with another `EnumDomain`:
    - exact raw bytes equal => same enum value.
    - otherwise error/no intersection.

## Exhaustive test checklist

Notation:

- `A + B -> C` means `A.AllOfMerge(B)` succeeds and returns `C`.
- `A + B -> error` means merge fails.
- `raw(x)` means an `EnumDomain` whose `RawMessage` bytes are exactly `x`.
- Exact set intersection preserves the left side order of surviving values.

### AllOfDomain

Valid cases:

- [ ] empty `AllOfDomain{}` + `String{MinLength:1}` -> `Domains:[String{MinLength:1}]`, `MergedDomain:String{MinLength:1}`.
- [ ] `AllOf{Domains:[String{MinLength:1}], Merged:String{MinLength:1}}` + `String{MaxLength:5}` -> `Domains:[String{MinLength:1}, String{MaxLength:5}]`, `Merged:String{MinLength:1, MaxLength:5}`.
- [ ] `AllOf{Merged:String{Enum:["a","b"]}}` + `String{Enum:["b","c"]}` -> merged string enum `['b']`.
- [ ] `AllOf{Merged:Number{Type:"number"}}` + `Number{Type:"integer"}` -> merged number type `integer`.
- [ ] `AllOf{Merged:Object{prop:a}}` + `Object{prop:b}` -> merged object with props `a,b` sorted by key.
- [ ] `AllOf{Merged:Array{MinItems:1}}` + `Array{MaxItems:3}` -> merged array `{MinItems:1, MaxItems:3}`.
- [ ] `AllOf{Merged:Bool{Enum:[true,false]}}` + `Bool{Enum:[false]}` -> merged bool enum `[false]`.
- [ ] `AllOf{Merged:String{Nullable:true}}` + `String{Nullable:true}` -> merged nullable true.
- [ ] `AllOf{Merged:String{Nullable:true}}` + `String{Nullable:false}` -> merged nullable false.
- [ ] `AllOf{Merged:String{Nullable:false}}` + `String{Nullable:true}` -> merged nullable false.
- [ ] `AllOf{Merged:String{Nullable:false}}` + `String{Nullable:false}` -> merged nullable false.
- [ ] `AllOf{Domains:[A], Merged:A}` + `AllOf{Domains:[B,C], Merged:B+C}` -> `Domains:[A,B,C]`, merged as `((A+B)+C)`.
- [ ] `String{MinLength:1}.AllOfMerge(AllOf{Domains:[String{MaxLength:5}], Merged:String{MaxLength:5}})` -> `AllOf` result with merged string `{MinLength:1, MaxLength:5}`.
- [ ] Parse/merge allOf containing only strings: `[String{MinLength:1}, String{MaxLength:5}]` -> merged string `{MinLength:1, MaxLength:5}`.
- [ ] Parse/merge allOf containing only numbers: `[Number{Type:"number"}, Number{Type:"integer"}]` -> merged number `{Type:"integer"}`.
- [ ] Parse/merge allOf containing only booleans: `[Bool{Enum:[true,false]}, Bool{Enum:[true]}]` -> merged bool enum `[true]`.
- [ ] Parse/merge allOf containing only arrays: `[Array{MinItems:1}, Array{MaxItems:3}]` -> merged array `{MinItems:1, MaxItems:3}`.
- [ ] Parse/merge allOf containing only objects: `[Object{prop:a}, Object{prop:b}]` -> merged object props `a,b`.
- [ ] Parse/merge nested allOf: `[String{MinLength:1}, AllOf[String{MaxLength:5}, String{Enum:["abc"]}]]` -> merged string `{MinLength:1, MaxLength:5, Enum:["abc"]}`.
- [ ] Parse/merge sibling constraints: `{type:string, maxLength:5, allOf:[{type:string, minLength:1}]}` -> merged string `{MinLength:1, MaxLength:5}`.
- [ ] allOf wrapper nullable absent + child `String{Nullable:true}` -> merged nullable true.
- [ ] allOf wrapper nullable true + child `String{Nullable:true}` -> merged nullable true.
- [ ] allOf wrapper nullable true + child `String{Nullable:false}` -> merged nullable false.
- [ ] allOf wrapper nullable false + child `String{Nullable:true}` -> merged nullable false.
- [ ] allOf wrapper nullable false + child `String{Nullable:false}` -> merged nullable false.

Invalid cases:

- [ ] `(*AllOfDomain)(nil) + String{}` -> error.
- [ ] `AllOf{} + nil` -> error.
- [ ] `AllOf{Merged:String{}} + Number{Type:"number"}` -> error.
- [ ] `AllOf{Merged:String{Enum:["a"]}} + String{Enum:["b"]}` -> error.
- [ ] `AllOf{Merged:Bool{Enum:[true]}} + Bool{Enum:[false]}` -> error.
- [ ] `AllOf{Merged:Array{Items:String{}}} + Array{Items:Bool{}}` -> error.
- [ ] `AllOf{Merged:Object{required prop:a, AdditionalFalse}} + Object{AdditionalFalse without prop:a}` -> error.
- [ ] `AllOf{Domains:[A], Merged:A}` + `AllOf{Domains:[B,bad], Merged:bad}` where merging `bad` fails -> error and original left allOf remains unchanged.
- [ ] `AllOf{Merged:String{Nullable:true}} + Number{Type:"integer", Nullable:true}` -> error, even though only `null` could overlap, because there is no `NullDomain`.
- [ ] Parse/merge allOf with incompatible primitive children `[String{}, Bool{}]` -> error.
- [ ] Parse/merge allOf with enum intersection empty -> error.
- [ ] Parse/merge allOf where any child parse fails -> error and no partial domain-store commit.

### StringDomain

Valid cases:

- [ ] `String{Nullable:false} + String{Nullable:false}` -> `String{Nullable:false}`.
- [ ] `String{Nullable:false} + String{Nullable:true}` -> `String{Nullable:false}`.
- [ ] `String{Nullable:true} + String{Nullable:false}` -> `String{Nullable:false}`.
- [ ] `String{Nullable:true} + String{Nullable:true}` -> `String{Nullable:true}`.
- [ ] `String{Enum:nil} + String{Enum:nil}` -> `Enum:nil`.
- [ ] `String{Enum:nil} + String{Enum:["a","b"]}` -> `Enum:["a","b"]`.
- [ ] `String{Enum:["a","b"]} + String{Enum:nil}` -> `Enum:["a","b"]`.
- [ ] `String{Enum:["a","b","c"]} + String{Enum:["b","c","d"]}` -> `Enum:["b","c"]`.
- [ ] `String{Enum:["b","a"]} + String{Enum:["a","b"]}` -> `Enum:["b","a"]`.
- [ ] `String{Enum:["a", raw(null)]} + String{Enum:[raw(null), "b"]}` -> enum containing only raw `null`.
- [ ] `String{Pattern:nil} + String{Pattern:nil}` -> `Pattern:nil`.
- [ ] `String{Pattern:nil} + String{Pattern:["p2"]}` -> `Pattern:["p2"]`.
- [ ] `String{Pattern:["p1"]} + String{Pattern:nil}` -> `Pattern:["p1"]`.
- [ ] `String{Pattern:["p1"]} + String{Pattern:["p2"]}` -> `Pattern:["p1","p2"]`.
- [ ] `String{Pattern:["p"]} + String{Pattern:["p"]}` -> `Pattern:["p","p"]`.
- [ ] `String{Format:nil} + String{Format:nil}` -> `Format:nil`.
- [ ] `String{Format:nil} + String{Format:["email"]}` -> `Format:["email"]`.
- [ ] `String{Format:["uuid"]} + String{Format:nil}` -> `Format:["uuid"]`.
- [ ] `String{Format:["email"]} + String{Format:["uuid"]}` -> `Format:["email","uuid"]`.
- [ ] `String{Format:["email"]} + String{Format:["email"]}` -> `Format:["email","email"]`.
- [ ] `String{XValidExamples:nil} + String{XValidExamples:nil}` -> `XValidExamples:nil`.
- [ ] `String{XValidExamples:nil} + String{XValidExamples:["a","b"]}` -> `XValidExamples:["a","b"]`.
- [ ] `String{XValidExamples:["a","b"]} + String{XValidExamples:nil}` -> `XValidExamples:["a","b"]`.
- [ ] `String{XValidExamples:["a","b","c"]} + String{XValidExamples:["b","c","d"]}` -> `XValidExamples:["b","c"]`.
- [ ] `String{XValidExamples:["b","a"]} + String{XValidExamples:["a","b"]}` -> `XValidExamples:["b","a"]`.
- [ ] `String{XValidExamples:["a"]} + String{XValidExamples:["b"]}` -> `XValidExamples:[]`, no error.
- [ ] `String{Enum:["a","b"]} + String{XValidExamples:["b","c"]}` -> `Enum:["b"]`, `XValidExamples:["b"]`.
- [ ] `String{Enum:["b","a"]} + String{XValidExamples:["a","b"]}` -> `Enum:["b","a"]`, `XValidExamples:["b","a"]`.
- [ ] `String{XInvalidExamples:nil} + String{XInvalidExamples:nil}` -> `XInvalidExamples:nil`.
- [ ] `String{XInvalidExamples:nil} + String{XInvalidExamples:["x"]}` -> `XInvalidExamples:["x"]`.
- [ ] `String{XInvalidExamples:["x"]} + String{XInvalidExamples:nil}` -> `XInvalidExamples:["x"]`.
- [ ] `String{XInvalidExamples:["a","b"]} + String{XInvalidExamples:["b","c"]}` -> `XInvalidExamples:["a","b","c"]`.
- [ ] `String{Enum:["ok"], XInvalidExamples:["ok"]} + String{}` -> keeps both `Enum:["ok"]` and `XInvalidExamples:["ok"]`.
- [ ] `String{MinLength:1} + String{MinLength:3}` -> `MinLength:3`.
- [ ] `String{MinLength:3} + String{MinLength:1}` -> `MinLength:3`.
- [ ] `String{MaxLength:nil} + String{MaxLength:5}` -> `MaxLength:5`.
- [ ] `String{MaxLength:5} + String{MaxLength:nil}` -> `MaxLength:5`.
- [ ] `String{MaxLength:9} + String{MaxLength:5}` -> `MaxLength:5`.
- [ ] `String{MinLength:10} + String{MaxLength:5}` -> `MinLength:10`, `MaxLength:5`, no error.
- [ ] `String{Pattern:["p1"], Format:["f1"], XValidExamples:["a","b"], XInvalidExamples:["x"], MinLength:1, MaxLength:10} + String{Pattern:["p2"], Format:["f2"], XValidExamples:["b","c"], XInvalidExamples:["y"], MinLength:2, MaxLength:8}` -> pattern `["p1","p2"]`, format `["f1","f2"]`, valid `["b"]`, invalid `["x","y"]`, min `2`, max `8`.
- [ ] `String{} + AllOf{Domains:[String{MinLength:1}], Merged:String{MinLength:1}}` -> allOf result with merged string.

Invalid cases:

- [ ] `(*StringDomain)(nil) + String{}` -> error.
- [ ] `String{} + nil` -> error.
- [ ] `String{} + Bool{}` -> error.
- [ ] `String{} + Number{Type:"number"}` -> error.
- [ ] `String{} + Array{}` -> error.
- [ ] `String{} + Object{}` -> error.
- [ ] `String{Enum:["a"]} + String{Enum:["b"]}` -> error.
- [ ] `String{Enum:["a"]} + String{Enum:["A"]}` -> error.
- [ ] `String{Enum:["a"]} + String{Enum:[raw(null)]}` -> error.
- [ ] `String{Enum:["a"]} + String{XValidExamples:["b"]}` -> error.
- [ ] `String{XValidExamples:["a"]} + String{Enum:["b"]}` -> error.
- [ ] `String{} + AllOf{Domains:[Bool{}], Merged:Bool{}}` -> error.
- [ ] any failed string merge leaves the left string unchanged.

### NumberDomain

Valid cases:

- [ ] `Number{Type:"number"} + Number{Type:"number"}` -> `Type:"number"`.
- [ ] `Number{Type:"integer"} + Number{Type:"integer"}` -> `Type:"integer"`.
- [ ] `Number{Type:"number"} + Number{Type:"integer"}` -> `Type:"integer"`.
- [ ] `Number{Type:"integer"} + Number{Type:"number"}` -> `Type:"integer"`.
- [ ] `Number{Nullable:false} + Number{Nullable:false}` -> `Nullable:false`.
- [ ] `Number{Nullable:false} + Number{Nullable:true}` -> `Nullable:false`.
- [ ] `Number{Nullable:true} + Number{Nullable:false}` -> `Nullable:false`.
- [ ] `Number{Nullable:true} + Number{Nullable:true}` -> `Nullable:true`.
- [ ] `Number{Enum:nil} + Number{Enum:nil}` -> `Enum:nil`.
- [ ] `Number{Enum:nil} + Number{Enum:[1,2]}` -> `Enum:[1,2]`.
- [ ] `Number{Enum:[1,2]} + Number{Enum:nil}` -> `Enum:[1,2]`.
- [ ] `Number{Enum:[1,2,1.0]} + Number{Enum:[2,1.0,3]}` -> `Enum:[2,1.0]`.
- [ ] `Number{Enum:[2,1]} + Number{Enum:[1,2]}` -> `Enum:[2,1]`.
- [ ] `Number{Enum:[1, raw(null)]} + Number{Enum:[raw(null), 2]}` -> enum containing only raw `null`.
- [ ] `Number{Minimum:nil} + Number{Minimum:1}` -> `Minimum:1`.
- [ ] `Number{Minimum:1} + Number{Minimum:nil}` -> `Minimum:1`.
- [ ] `Number{Minimum:1} + Number{Minimum:2}` -> `Minimum:2`.
- [ ] `Number{Minimum:2} + Number{Minimum:1}` -> `Minimum:2`.
- [ ] `Number{Minimum:1, ExclusiveMinimum:false} + Number{Minimum:1, ExclusiveMinimum:true}` -> `Minimum:1`, `ExclusiveMinimum:true`.
- [ ] `Number{Minimum:1, ExclusiveMinimum:true} + Number{Minimum:1, ExclusiveMinimum:false}` -> `Minimum:1`, `ExclusiveMinimum:true`.
- [ ] `Number{Minimum:1.0, ExclusiveMinimum:false} + Number{Minimum:1, ExclusiveMinimum:true}` -> keeps left lexeme `1.0`, `ExclusiveMinimum:true`.
- [ ] `Number{Minimum:1e2} + Number{Minimum:99}` -> keeps lexeme `1e2`.
- [ ] `Number{Minimum:-5} + Number{Minimum:-2}` -> `Minimum:-2`.
- [ ] `Number{Maximum:nil} + Number{Maximum:10}` -> `Maximum:10`.
- [ ] `Number{Maximum:10} + Number{Maximum:nil}` -> `Maximum:10`.
- [ ] `Number{Maximum:10} + Number{Maximum:8}` -> `Maximum:8`.
- [ ] `Number{Maximum:8} + Number{Maximum:10}` -> `Maximum:8`.
- [ ] `Number{Maximum:1, ExclusiveMaximum:false} + Number{Maximum:1, ExclusiveMaximum:true}` -> `Maximum:1`, `ExclusiveMaximum:true`.
- [ ] `Number{Maximum:1, ExclusiveMaximum:true} + Number{Maximum:1, ExclusiveMaximum:false}` -> `Maximum:1`, `ExclusiveMaximum:true`.
- [ ] `Number{Maximum:1.0, ExclusiveMaximum:false} + Number{Maximum:1, ExclusiveMaximum:true}` -> keeps left lexeme `1.0`, `ExclusiveMaximum:true`.
- [ ] `Number{Maximum:1e2} + Number{Maximum:99}` -> `Maximum:99`.
- [ ] `Number{Minimum:10} + Number{Maximum:5}` -> `Minimum:10`, `Maximum:5`, no error.
- [ ] `Number{Minimum:5, ExclusiveMinimum:true} + Number{Maximum:5, ExclusiveMaximum:true}` -> both bounds stored, no error.
- [ ] `Number{MultipleOf:nil} + Number{MultipleOf:2}` -> `MultipleOf:2`.
- [ ] `Number{MultipleOf:2} + Number{MultipleOf:nil}` -> `MultipleOf:2`.
- [ ] `Number{MultipleOf:2} + Number{MultipleOf:3}` -> `MultipleOf:6`.
- [ ] `Number{MultipleOf:0.5} + Number{MultipleOf:0.25}` -> `MultipleOf:0.5`.
- [ ] `Number{MultipleOf:1.5} + Number{MultipleOf:2.5}` -> `MultipleOf:7.5`.
- [ ] `Number{Type:"number", MultipleOf:2.5} + Number{Type:"integer"}` -> `Type:"integer"`, `MultipleOf:2.5`, no integer normalization.
- [ ] `Number{Format:nil} + Number{Format:nil}` -> `Format:nil`.
- [ ] `Number{Type:"number", Format:nil} + Number{Type:"number", Format:"float"}` -> `Format:"float"`.
- [ ] `Number{Type:"number", Format:"double"} + Number{Type:"number", Format:nil}` -> `Format:"double"`.
- [ ] `Number{Type:"number", Format:"float"} + Number{Type:"number", Format:"float"}` -> `Format:"float"`.
- [ ] `Number{Type:"integer", Format:"int32"} + Number{Type:"integer", Format:"int32"}` -> `Format:"int32"`.
- [ ] `Number{Type:"number", Format:nil} + Number{Type:"integer", Format:"int64"}` -> `Type:"integer"`, `Format:"int64"`.
- [ ] all fields together: `Number{Type:"number", Nullable:true, Enum:[1,2], Minimum:0, Maximum:10, MultipleOf:2, Format:nil} + Number{Type:"integer", Nullable:true, Enum:[2,3], Minimum:1, Maximum:8, MultipleOf:4, Format:"int64"}` -> `Type:"integer"`, `Nullable:true`, `Enum:[2]`, `Minimum:1`, `Maximum:8`, `MultipleOf:4`, `Format:"int64"`.
- [ ] `Number{} + AllOf{Domains:[Number{Type:"number", Minimum:1}], Merged:Number{Type:"number", Minimum:1}}` -> allOf result with merged number.

Invalid cases:

- [ ] `(*NumberDomain)(nil) + Number{Type:"number"}` -> error.
- [ ] `Number{Type:"number"} + nil` -> error.
- [ ] `Number{Type:"number"} + String{}` -> error.
- [ ] `Number{Type:"number"} + Bool{}` -> error.
- [ ] `Number{Type:"number"} + Array{}` -> error.
- [ ] `Number{Type:"number"} + Object{}` -> error.
- [ ] `Number{Type:""} + Number{Type:"number"}` -> error.
- [ ] `Number{Type:"number"} + Number{Type:""}` -> error.
- [ ] `Number{Type:"string"} + Number{Type:"number"}` -> error.
- [ ] `Number{Type:"number"} + Number{Type:"string"}` -> error.
- [ ] `Number{Enum:[1]} + Number{Enum:[1.0]}` -> error.
- [ ] `Number{Enum:[1]} + Number{Enum:[2]}` -> error.
- [ ] `Number{Enum:[1]} + Number{Enum:[raw(null)]}` -> error.
- [ ] `Number{Minimum:Number("bad")}` + `Number{Minimum:1}` -> error.
- [ ] `Number{Maximum:Number("bad")}` + `Number{Maximum:1}` -> error.
- [ ] `Number{MultipleOf:Number("bad")}` + `Number{MultipleOf:2}` -> error.
- [ ] `Number{Type:"number", Format:"float"} + Number{Type:"number", Format:"double"}` -> error.
- [ ] `Number{Type:"integer", Format:"int32"} + Number{Type:"integer", Format:"int64"}` -> error.
- [ ] `Number{Type:"number", Format:"float"} + Number{Type:"integer"}` -> error.
- [ ] `Number{Type:"integer"} + Number{Type:"number", Format:"float"}` -> error.
- [ ] `Number{Type:"number", Format:"double"} + Number{Type:"integer"}` -> error.
- [ ] `Number{Type:"integer"} + Number{Type:"number", Format:"double"}` -> error.
- [ ] `Number{Type:"number", Format:"float"} + Number{Type:"integer", Format:"int32"}` -> error.
- [ ] `Number{Type:"integer", Format:"int32"} + Number{Type:"number", Format:"float"}` -> error.
- [ ] `Number{Type:"number"} + AllOf{Domains:[String{}], Merged:String{}}` -> error.
- [ ] any failed number merge leaves the left number unchanged.

### BoolDomain

Valid cases:

- [ ] `Bool{Nullable:false} + Bool{Nullable:false}` -> `Nullable:false`.
- [ ] `Bool{Nullable:false} + Bool{Nullable:true}` -> `Nullable:false`.
- [ ] `Bool{Nullable:true} + Bool{Nullable:false}` -> `Nullable:false`.
- [ ] `Bool{Nullable:true} + Bool{Nullable:true}` -> `Nullable:true`.
- [ ] `Bool{Enum:nil} + Bool{Enum:nil}` -> `Enum:nil`.
- [ ] `Bool{Enum:nil} + Bool{Enum:[true,false]}` -> `Enum:[true,false]`.
- [ ] `Bool{Enum:[true,false]} + Bool{Enum:nil}` -> `Enum:[true,false]`.
- [ ] `Bool{Enum:[true,false]} + Bool{Enum:[false]}` -> `Enum:[false]`.
- [ ] `Bool{Enum:[false,true]} + Bool{Enum:[true,false]}` -> `Enum:[false,true]`.
- [ ] `Bool{Enum:[true, raw(null)]} + Bool{Enum:[raw(null), false]}` -> enum containing only raw `null`.
- [ ] `Bool{Nullable:true, Enum:[true,false]} + Bool{Nullable:true, Enum:[false]}` -> `Nullable:true`, `Enum:[false]`.
- [ ] `Bool{} + AllOf{Domains:[Bool{Enum:[true]}], Merged:Bool{Enum:[true]}}` -> allOf result with merged bool.

Invalid cases:

- [ ] `(*BoolDomain)(nil) + Bool{}` -> error.
- [ ] `Bool{} + nil` -> error.
- [ ] `Bool{} + String{}` -> error.
- [ ] `Bool{} + Number{Type:"number"}` -> error.
- [ ] `Bool{} + Array{}` -> error.
- [ ] `Bool{} + Object{}` -> error.
- [ ] `Bool{Enum:[true]} + Bool{Enum:[false]}` -> error.
- [ ] `Bool{Enum:[true]} + Bool{Enum:[raw(null)]}` -> error.
- [ ] `Bool{} + AllOf{Domains:[String{}], Merged:String{}}` -> error.
- [ ] any failed bool merge leaves the left bool unchanged.

### ArrayDomain

Valid cases:

- [ ] `Array{Nullable:false} + Array{Nullable:false}` -> `Nullable:false`.
- [ ] `Array{Nullable:false} + Array{Nullable:true}` -> `Nullable:false`.
- [ ] `Array{Nullable:true} + Array{Nullable:false}` -> `Nullable:false`.
- [ ] `Array{Nullable:true} + Array{Nullable:true}` -> `Nullable:true`.
- [ ] `Array{Enum:nil} + Array{Enum:nil}` -> `Enum:nil`.
- [ ] `Array{Enum:nil} + Array{Enum:[raw(["a"]), raw(["b"])]}` -> same right enum set.
- [ ] `Array{Enum:[raw(["a"]), raw(["b"])]} + Array{Enum:nil}` -> same left enum set.
- [ ] `Array{Enum:[raw(["a"]), raw(["b"]), raw(["c"])]} + Array{Enum:[raw(["b"]), raw(["c"]), raw(["d"])]}` -> `Enum:[raw(["b"]), raw(["c"])]`.
- [ ] `Array{Enum:[raw(["b"]), raw(["a"])]} + Array{Enum:[raw(["a"]), raw(["b"])]}` -> `Enum:[raw(["b"]), raw(["a"])]`.
- [ ] `Array{Enum:[raw(null), raw(["a"])]} + Array{Enum:[raw(["b"]), raw(null)]}` -> `Enum:[raw(null)]`.
- [ ] `Array{Enum:[raw(["too-long"])] , MinItems:99} + Array{MaxItems:0}` -> keeps enum and stores min/max; enum is not filtered.
- [ ] `Array{Items:nil} + Array{Items:nil}` -> `Items:nil`.
- [ ] `Array{Items:nil} + Array{Items:String{MinLength:1}}` -> `Items:String{MinLength:1}`.
- [ ] `Array{Items:String{MinLength:1}} + Array{Items:nil}` -> `Items:String{MinLength:1}`.
- [ ] `Array{Items:String{MinLength:1}} + Array{Items:String{MaxLength:5}}` -> `Items:String{MinLength:1, MaxLength:5}`.
- [ ] `Array{Items:Number{Type:"number"}} + Array{Items:Number{Type:"integer"}}` -> `Items:Number{Type:"integer"}`.
- [ ] `Array{MinItems:1} + Array{MinItems:3}` -> `MinItems:3`.
- [ ] `Array{MinItems:3} + Array{MinItems:1}` -> `MinItems:3`.
- [ ] `Array{MaxItems:nil} + Array{MaxItems:5}` -> `MaxItems:5`.
- [ ] `Array{MaxItems:5} + Array{MaxItems:nil}` -> `MaxItems:5`.
- [ ] `Array{MaxItems:9} + Array{MaxItems:5}` -> `MaxItems:5`.
- [ ] `Array{MinItems:10} + Array{MaxItems:5}` -> `MinItems:10`, `MaxItems:5`, no error.
- [ ] all fields together: `Array{Nullable:true, Enum:[raw(["a"]),raw(["b"])], Items:String{MinLength:1}, MinItems:1, MaxItems:10} + Array{Nullable:true, Enum:[raw(["b"]),raw(["c"])], Items:String{MaxLength:5}, MinItems:2, MaxItems:8}` -> nullable true, enum `[raw(["b"])]`, items `String{MinLength:1, MaxLength:5}`, min `2`, max `8`.
- [ ] `Array{} + AllOf{Domains:[Array{MinItems:1}], Merged:Array{MinItems:1}}` -> allOf result with merged array.

Invalid cases:

- [ ] `(*ArrayDomain)(nil) + Array{}` -> error.
- [ ] `Array{} + nil` -> error.
- [ ] `Array{} + String{}` -> error.
- [ ] `Array{} + Number{Type:"number"}` -> error.
- [ ] `Array{} + Bool{}` -> error.
- [ ] `Array{} + Object{}` -> error.
- [ ] `Array{Enum:[raw([1])]} + Array{Enum:[raw([1.0])]}` -> error.
- [ ] `Array{Enum:[raw(["a"])]} + Array{Enum:[raw(["b"])]}` -> error.
- [ ] `Array{Enum:[raw({"a":1,"b":2})]} + Array{Enum:[raw({"b":2,"a":1})]}` -> error because raw bytes differ.
- [ ] `Array{Enum:[raw(["a"])]} + Array{Enum:[raw(null)]}` -> error.
- [ ] `Array{Enum:[non-EnumDomain]}` + `Array{Enum:[raw([])]}` -> error.
- [ ] `Array{Enum:[EnumDomain{RawMessage:nil}]}` + `Array{Enum:[raw([])]}` -> error.
- [ ] `Array{Items:String{}} + Array{Items:Bool{}}` -> error.
- [ ] `Array{} + AllOf{Domains:[String{}], Merged:String{}}` -> error.
- [ ] any failed array merge leaves the left array unchanged.

### ObjectDomain

Valid cases:

- [ ] `Object{Nullable:false} + Object{Nullable:false}` -> `Nullable:false`.
- [ ] `Object{Nullable:false} + Object{Nullable:true}` -> `Nullable:false`.
- [ ] `Object{Nullable:true} + Object{Nullable:false}` -> `Nullable:false`.
- [ ] `Object{Nullable:true} + Object{Nullable:true}` -> `Nullable:true`.
- [ ] `Object{Enum:nil} + Object{Enum:nil}` -> `Enum:nil`.
- [ ] `Object{Enum:nil} + Object{Enum:[raw({"a":1}), raw({"b":2})]}` -> same right enum set.
- [ ] `Object{Enum:[raw({"a":1}), raw({"b":2})]} + Object{Enum:nil}` -> same left enum set.
- [ ] `Object{Enum:[raw({"a":1}), raw({"b":2}), raw({"c":3})]} + Object{Enum:[raw({"b":2}), raw({"c":3}), raw({"d":4})]}` -> `Enum:[raw({"b":2}), raw({"c":3})]`.
- [ ] `Object{Enum:[raw({"b":2}), raw({"a":1})]} + Object{Enum:[raw({"a":1}), raw({"b":2})]}` -> `Enum:[raw({"b":2}), raw({"a":1})]`.
- [ ] `Object{Enum:[raw(null), raw({"a":1})]} + Object{Enum:[raw({"b":2}), raw(null)]}` -> `Enum:[raw(null)]`.
- [ ] `Object{Enum:[raw({"forbidden":true})], AdditionalFalse} + Object{}` -> enum kept; enum is not filtered by object constraints.
- [ ] `Object{Properties:[a optional nil], AdditionalTrue} + Object{Properties:[b optional nil], AdditionalTrue}` -> props `a,b`, both optional, sorted.
- [ ] `Object{Properties:[b optional nil], AdditionalTrue} + Object{Properties:[a optional nil], AdditionalTrue}` -> props `a,b`, sorted.
- [ ] `Object{Properties:[a optional nil]} + Object{Properties:[a optional nil]}` -> prop `a` optional nil.
- [ ] `Object{Properties:[a required nil]} + Object{Properties:[a optional nil]}` -> prop `a` required nil.
- [ ] `Object{Properties:[a optional nil]} + Object{Properties:[a required nil]}` -> prop `a` required nil.
- [ ] `Object{Properties:[a required nil]} + Object{Properties:[a required nil]}` -> prop `a` required nil.
- [ ] `Object{Properties:[a optional String{MinLength:1}]} + Object{Properties:[a optional String{MaxLength:5}]}` -> prop `a` optional `String{MinLength:1, MaxLength:5}`.
- [ ] `Object{Properties:[a required String{MinLength:1}]} + Object{Properties:[a optional String{MaxLength:5}]}` -> prop `a` required `String{MinLength:1, MaxLength:5}`.
- [ ] `Object{Properties:[a optional nil]} + Object{Properties:[a optional String{MinLength:1}]}` -> prop `a` optional `String{MinLength:1}`.
- [ ] `Object{Properties:[a optional String{MinLength:1}]} + Object{Properties:[a optional nil]}` -> prop `a` optional `String{MinLength:1}`.
- [ ] `Object{Properties:[a optional String{}], AdditionalFalse} + Object{Properties:[a optional String{MinLength:1}], AdditionalFalse}` -> prop `a` merged, final `AdditionalFalse`.
- [ ] left prop `a` optional, right `AdditionalTrue` -> keep prop `a`.
- [ ] left prop `a` required, right `AdditionalTrue` -> keep required prop `a`.
- [ ] right prop `a` optional, left `AdditionalTrue` -> keep prop `a`.
- [ ] right prop `a` required, left `AdditionalTrue` -> keep required prop `a`.
- [ ] left prop `a` optional, right `AdditionalFalse` without `a` -> drop prop `a`, final `AdditionalFalse`.
- [ ] right prop `a` optional, left `AdditionalFalse` without `a` -> drop prop `a`, final `AdditionalFalse`.
- [ ] both sides `AdditionalFalse` with disjoint optional props `a` and `b` -> drop both optional props, final no props, `AdditionalFalse`.
- [ ] left prop `a` optional `String{MinLength:1}`, right `AdditionalSchema:String{MaxLength:5}` -> prop `a` domain `String{MinLength:1, MaxLength:5}`.
- [ ] right prop `a` optional `String{MinLength:1}`, left `AdditionalSchema:String{MaxLength:5}` -> prop `a` domain `String{MinLength:1, MaxLength:5}`.
- [ ] left prop `a` required `String{MinLength:1}`, right `AdditionalSchema:String{MaxLength:5}` -> prop `a` required `String{MinLength:1, MaxLength:5}`.
- [ ] left prop `a` optional nil, right `AdditionalSchema:String{MaxLength:5}` -> prop `a` domain `String{MaxLength:5}`.
- [ ] left prop `a` required nil, right `AdditionalSchema:String{MaxLength:5}` -> prop `a` required with domain `String{MaxLength:5}`.
- [ ] right prop `a` optional nil, left `AdditionalSchema:String{MaxLength:5}` -> prop `a` domain `String{MaxLength:5}`.
- [ ] right prop `a` required nil, left `AdditionalSchema:String{MaxLength:5}` -> prop `a` required with domain `String{MaxLength:5}`.
- [ ] `Object{AdditionalTrue} + Object{AdditionalTrue}` -> `AdditionalTrue`.
- [ ] `Object{AdditionalTrue} + Object{AdditionalFalse}` -> `AdditionalFalse`.
- [ ] `Object{AdditionalFalse} + Object{AdditionalTrue}` -> `AdditionalFalse`.
- [ ] `Object{AdditionalFalse} + Object{AdditionalFalse}` -> `AdditionalFalse`.
- [ ] `Object{AdditionalTrue} + Object{AdditionalSchema:String{MaxLength:5}}` -> `AdditionalSchema:String{MaxLength:5}`.
- [ ] `Object{AdditionalSchema:String{MaxLength:5}} + Object{AdditionalTrue}` -> `AdditionalSchema:String{MaxLength:5}`.
- [ ] `Object{AdditionalFalse} + Object{AdditionalSchema:String{MaxLength:5}}` -> `AdditionalFalse` when no explicit cross-property needs schema application.
- [ ] `Object{AdditionalSchema:String{MinLength:1}} + Object{AdditionalSchema:String{MaxLength:5}}` -> `AdditionalSchema:String{MinLength:1, MaxLength:5}`.
- [ ] `Object{MinProps:1} + Object{MinProps:3}` -> `MinProps:3`.
- [ ] `Object{MinProps:3} + Object{MinProps:1}` -> `MinProps:3`.
- [ ] `Object{MaxProps:nil} + Object{MaxProps:5}` -> `MaxProps:5`.
- [ ] `Object{MaxProps:5} + Object{MaxProps:nil}` -> `MaxProps:5`.
- [ ] `Object{MaxProps:9} + Object{MaxProps:5}` -> `MaxProps:5`.
- [ ] `Object{MinProps:10} + Object{MaxProps:5}` -> `MinProps:10`, `MaxProps:5`, no error.
- [ ] `Object{Properties:[a required, b required], MaxProps:nil} + Object{MaxProps:1}` -> keeps required props and `MaxProps:1`, no count error.
- [ ] `Object{Properties:[a optional], AdditionalFalse, MinProps:5} + Object{}` -> no count/enough-properties error.
- [ ] all fields together: nullable true on both, enum intersection one raw value, same prop `a` merged, optional prop `b` dropped by other `AdditionalFalse`, min takes larger, max takes smaller -> expected merged object with only allowed/merged props and stricter min/max.
- [ ] `Object{} + AllOf{Domains:[Object{MinProps:1}], Merged:Object{MinProps:1}}` -> allOf result with merged object.

Invalid cases:

- [ ] `(*ObjectDomain)(nil) + Object{}` -> error.
- [ ] `Object{} + nil` -> error.
- [ ] `Object{} + String{}` -> error.
- [ ] `Object{} + Number{Type:"number"}` -> error.
- [ ] `Object{} + Bool{}` -> error.
- [ ] `Object{} + Array{}` -> error.
- [ ] `Object{Enum:[raw({"a":1})]} + Object{Enum:[raw({"a":2})]}` -> error.
- [ ] `Object{Enum:[raw({"a":1,"b":2})]} + Object{Enum:[raw({"b":2,"a":1})]}` -> error because raw bytes differ.
- [ ] `Object{Enum:[raw({"a":1})]} + Object{Enum:[raw(null)]}` -> error.
- [ ] `Object{Enum:[non-EnumDomain]}` + `Object{Enum:[raw({})]}` -> error.
- [ ] `Object{Enum:[EnumDomain{RawMessage:nil}]}` + `Object{Enum:[raw({})]}` -> error.
- [ ] property entry that is not `*Property` on left -> error.
- [ ] property entry that is not `*Property` on right -> error.
- [ ] same property key with incompatible domains: `Object{a:String{}} + Object{a:Bool{}}` -> error.
- [ ] left required prop `a`, right `AdditionalFalse` without `a` -> error.
- [ ] right required prop `a`, left `AdditionalFalse` without `a` -> error.
- [ ] left prop `a` String, right `AdditionalSchema:Bool{}` -> error.
- [ ] right prop `a` String, left `AdditionalSchema:Bool{}` -> error.
- [ ] `Object{AdditionalSchema:nil} + Object{AdditionalTrue}` -> error.
- [ ] `Object{AdditionalTrue} + Object{AdditionalSchema:nil}` -> error.
- [ ] `Object{AdditionalSchema:String{}} + Object{AdditionalSchema:Bool{}}` -> error.
- [ ] `Object{AdditionalSchema:failingDomain} + Object{AdditionalSchema:String{}}` -> error.
- [ ] `Object{} + AllOf{Domains:[String{}], Merged:String{}}` -> error.
- [ ] any failed object merge leaves the left object unchanged.

### EnumDomain

Valid cases:

- [ ] `Enum{raw("\"a\"")} + Enum{raw("\"a\"")}` -> same enum value.
- [ ] `Enum{raw("null")} + Enum{raw("null")}` -> same raw null enum value.
- [ ] `Enum{raw("1")} + Enum{raw("1")}` -> same enum value.
- [ ] `Enum{raw("[1,2]")} + Enum{raw("[1,2]")}` -> same enum value.
- [ ] `Enum{raw("{\"a\":1}")} + Enum{raw("{\"a\":1}")}` -> same enum value.

Invalid cases:

- [ ] `(*EnumDomain)(nil) + Enum{raw("null")}` -> error.
- [ ] `Enum{raw("null")} + nil` -> error.
- [ ] `Enum{RawMessage:nil} + Enum{raw("null")}` -> error.
- [ ] `Enum{raw("\"a\"")} + Enum{raw("\"b\"")}` -> error.
- [ ] `Enum{raw("1")} + Enum{raw("1.0")}` -> error.
- [ ] `Enum{raw("{\"a\":1,\"b\":2}")} + Enum{raw("{\"b\":2,\"a\":1}")}` -> error.
- [ ] `Enum{raw("{\"a\":1}")} + Enum{raw("{ \"a\": 1 }")}` -> error.
- [ ] `Enum{raw("null")} + Enum{raw("\"null\"")}` -> error.
- [ ] `Enum{raw("null")} + String{}` -> error.
- [ ] `Enum{raw("null")} + AllOf{Domains:[Enum{raw("null")}], Merged:Enum{raw("null")}}` -> error; enum domains are not general schema merge domains.
- [ ] any failed enum comparison leaves the left enum unchanged.
