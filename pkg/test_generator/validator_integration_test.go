package testgenerator

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"decode_and_validate_generator/pkg/internal/oas"
	"decode_and_validate_generator/pkg/test_generator/internal/suite"
	"pgregory.net/rapid"
)

// requestBodyValidator is the private seam shared by all independent request validators.
type requestBodyValidator interface {
	Validate(body []byte) error
}

// validatorAdapter supplies the seam implementation and its pinned-library diagnostic name.
type validatorAdapter struct {
	name      string
	validator requestBodyValidator
	cleanup   func()
}

// bodyRejectionError distinguishes a schema verdict from adapter setup or routing failures.
type bodyRejectionError struct {
	err error
}

// Error reports the validator's full rejection diagnostic.
func (rejection bodyRejectionError) Error() string {
	return rejection.err.Error()
}

// Unwrap preserves the validator's structured error.
func (rejection bodyRejectionError) Unwrap() error {
	return rejection.err
}

// validatorOutcome records one adapter's verdict for the exact shared body.
type validatorOutcome struct {
	name string
	err  error
}

// validatorAdapterFactory constructs one adapter after loading one fixture document.
type validatorAdapterFactory func([]byte) (validatorAdapter, error)

// TestCheckJSONRequestBodyAgainstValidators runs every generated CasePlan against all independent adapters.
func TestCheckJSONRequestBodyAgainstValidators(t *testing.T) {
	t.Parallel()

	fixtures := validatorCorpus()
	if err := validateValidatorCorpus(fixtures); err != nil {
		t.Fatal(err)
	}

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			spec := fixture.spec()

			compiled, err := compileValidatorFixture(spec)
			if err != nil {
				t.Fatalf("compile generator fixture %s (%s): %v", fixture.ID, fixture.Category, err)
			}

			adapters, err := newValidatorAdapters(spec)
			if err != nil {
				t.Fatalf("set up validator adapters for fixture %s (%s): %v", fixture.ID, fixture.Category, err)
			}

			t.Cleanup(func() {
				releaseValidatorAdapters(adapters)
			})

			runValidatorCasePlans(t, fixture, compiled, adapters)
		})
	}
}

// compileValidatorFixture compiles all CasePlans once, before any adapter sees a generated body.
func compileValidatorFixture(spec []byte) (*suite.CompiledSuite, error) {
	source, err := oas.Parse(spec, "checkThing")
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI source: %w", err)
	}

	compiled, err := suite.NewCompiler(source).CompileSuite(suite.MustHaveAllXValidCases)
	if err != nil {
		return nil, fmt.Errorf("compile CasePlans: %w", err)
	}

	if len(compiled.Cases) == 0 {
		return nil, fmt.Errorf("compile CasePlans: fixture has no CasePlans")
	}

	return compiled, nil
}

// newValidatorAdapters creates all independently loaded validators before any body is checked.
func newValidatorAdapters(spec []byte) ([]validatorAdapter, error) {
	return newValidatorAdaptersFromFactories(spec, []validatorAdapterFactory{
		newRuntimeValidationRequestBodyValidator,
		newLibopenapiRequestBodyValidator,
		newKinopenapiRequestBodyValidator,
	})
}

// newExternalValidatorAdapters creates only the pinned third-party validators.
func newExternalValidatorAdapters(spec []byte) ([]validatorAdapter, error) {
	return newValidatorAdaptersFromFactories(spec, []validatorAdapterFactory{
		newLibopenapiRequestBodyValidator,
		newKinopenapiRequestBodyValidator,
	})
}

// newValidatorAdaptersFromFactories constructs and validates each requested adapter.
func newValidatorAdaptersFromFactories(
	spec []byte,
	factories []validatorAdapterFactory,
) ([]validatorAdapter, error) {
	adapters := make([]validatorAdapter, 0, len(factories))

	for _, factory := range factories {
		adapter, err := factory(spec)
		if err != nil {
			cleanupValidatorAdapter(adapter)

			releaseValidatorAdapters(adapters)

			return nil, err
		}

		if adapter.name == "" {
			cleanupValidatorAdapter(adapter)
			releaseValidatorAdapters(adapters)

			return nil, fmt.Errorf("validator adapter from factory has no name")
		}

		if adapter.validator == nil {
			cleanupValidatorAdapter(adapter)

			releaseValidatorAdapters(adapters)

			return nil, fmt.Errorf("validator adapter %q has no validator", adapter.name)
		}

		adapters = append(adapters, adapter)
	}

	return adapters, nil
}

// cleanupValidatorAdapter releases an adapter that owns a libopenapi validator.
func cleanupValidatorAdapter(adapter validatorAdapter) {
	if adapter.cleanup != nil {
		adapter.cleanup()
	}
}

// releaseValidatorAdapters releases the loaded libopenapi adapter after one fixture completes.
func releaseValidatorAdapters(adapters []validatorAdapter) {
	for index := range adapters {
		cleanupValidatorAdapter(adapters[index])
	}
}

// runValidatorCasePlans draws each body once and gives its exact bytes to all adapters.
func runValidatorCasePlans(
	t *testing.T,
	fixture validatorCorpusFixture,
	compiled *suite.CompiledSuite,
	adapters []validatorAdapter,
) {
	t.Helper()

	for index, plannedCase := range compiled.Cases {
		t.Run(fmt.Sprintf("case-%03d", index+1), rapid.MakeCheck(func(rt *rapid.T) {
			body, err := drawPlannedBody(rt, plannedCase)
			if err != nil {
				rt.Fatalf("%s", validatorCaseFailure(
					fixture,
					plannedCase,
					[]validatorOutcome{{name: "generator", err: err}},
					nil,
				))
			}

			outcomes := make([]validatorOutcome, 0, len(adapters))
			for _, adapter := range adapters {
				outcomes = append(outcomes, validatorOutcome{
					name: adapter.name,
					err:  adapter.validator.Validate(body),
				})
			}

			for _, outcome := range outcomes {
				if !validatorVerdictMatches(plannedCase.Expect, outcome.err) {
					rt.Fatalf("%s", validatorCaseFailure(fixture, plannedCase, outcomes, body))
				}
			}
		}))
	}
}

// drawPlannedBody draws and marshals one exact JSON body for a CasePlan.
func drawPlannedBody(rt *rapid.T, plannedCase suite.CasePlan) ([]byte, error) {
	value := plannedCase.Generator.Draw(rt, "json value")

	body, err := value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("encode generated JSON: %w", err)
	}

	return body, nil
}

// validatorVerdictMatches checks each adapter directly against the generator's contract.
func validatorVerdictMatches(expect suite.ExpectedResult, validationErr error) bool {
	switch expect {
	case suite.ExpectAccepted:
		return validationErr == nil
	case suite.ExpectRejected:
		return isBodyRejection(validationErr)
	default:
		return false
	}
}

// isBodyRejection reports whether an error is an actual request-body schema verdict.
func isBodyRejection(err error) bool {
	var rejection bodyRejectionError

	return errors.As(err, &rejection)
}

// validatorCaseFailure includes the fixture, CasePlan, exact body, and adapter needed to replay a disagreement.
func validatorCaseFailure(
	fixture validatorCorpusFixture,
	plannedCase suite.CasePlan,
	outcomes []validatorOutcome,
	body []byte,
) string {
	var diagnostics string
	for _, outcome := range outcomes {
		diagnostics += fmt.Sprintf(
			"\nadapter: %s\nbody rejection: %t\nerror: %v",
			outcome.name,
			isBodyRejection(outcome.err),
			outcome.err,
		)
	}

	return fmt.Sprintf(
		"fixture ID: %s\ncategory: %s\nCasePlan: %s\n"+
			"source: %s\nkeyword: %s\nexpectation: %s\n"+
			"canonical JSON: %s%s",
		fixture.ID,
		fixture.Category,
		plannedCase.Name,
		plannedCase.Source.Pointer,
		plannedCase.Source.Keyword,
		casePlanExpectation(plannedCase.Expect),
		body,
		diagnostics,
	)
}

// casePlanExpectation returns the explicit generator contract used in mismatch diagnostics.
func casePlanExpectation(expect suite.ExpectedResult) string {
	switch expect {
	case suite.ExpectAccepted:
		return "accepted"
	case suite.ExpectRejected:
		return "rejected"
	default:
		return fmt.Sprintf("unknown (%d)", expect)
	}
}

// validatorPhase is the exact setup/body outcome pinned by one characterization row.
type validatorPhase uint8

const (
	// validatorBodyAccepted expects successful construction and body validation.
	validatorBodyAccepted validatorPhase = iota
	// validatorBodyRejected expects successful construction and body rejection.
	validatorBodyRejected
	// validatorSetupRejected expects the adapter constructor to reject the document.
	validatorSetupRejected
)

// validatorCharacterization isolates known, pinned library behavior outside the consensus corpus.
type validatorCharacterization struct {
	ID                      string
	Schema                  string
	Components              string
	Body                    []byte
	Runtime                 validatorPhase
	RuntimeErrorContains    string
	Libopenapi              validatorPhase
	LibopenapiErrorContains string
	Kinopenapi              validatorPhase
	KinopenapiErrorContains string
}

// validatorCharacterizations contains no allowlist: each row pins one body and one exact adapter phase.
//
//nolint:maintidx // Keeping all fixed cross-validator expectations together makes omissions visible.
func validatorCharacterizations() []validatorCharacterization {
	return []validatorCharacterization{
		{
			ID: "adjacent-integers-above-2^53",
			Schema: `
      type: integer
      minimum: 9007199254740993
      maximum: 9007199254740993
`,
			Body:                 []byte(`9007199254740992`),
			Runtime:              validatorBodyRejected,
			RuntimeErrorContains: "keyword minimum",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
			// libopenapi-validator v0.13.13 and kin-openapi v0.140.0 both round schema and body numbers to float64.
			// Wright draft-00 §4.2 defines JSON numbers as arbitrary-precision base-10 decimals, so this body is invalid.
		},
		{
			ID: "safe-exponent-spelling",
			Schema: `
      type: number
      minimum: 100
      maximum: 100
`,
			Body:       []byte(`1e2`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
			// Wright draft-00 §4.2 defines numbers by mathematical value, independent of JSON spelling.
		},
		{
			ID: "negative-zero-spelling",
			Schema: `
      type: number
      minimum: 0
      maximum: 0
`,
			Body:       []byte(`-0`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
			// Wright draft-00 §4.3 makes numerically equal spellings equal, so negative zero satisfies these bounds.
		},
		{
			ID: "finite-json-number-outside-float64",
			Schema: `
      type: number
`,
			Body:                    []byte(`1e400`),
			Libopenapi:              validatorBodyRejected,
			LibopenapiErrorContains: "cannot unmarshal number 1e400 into Go value of type float64",
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "strconv.ParseFloat: parsing \"1e400\": value out of range",
			// Both pinned adapters reject finite JSON 1e400 while decoding to float64.
			// Wright draft-00 §4.2 has no float64 limit.
		},
		{
			ID: "invalid-email-format-policy",
			Schema: `
      type: string
      format: email
      x-valid-examples: [a@example.com]
`,
			Body:                 []byte(`"not-an-email"`),
			Runtime:              validatorBodyRejected,
			RuntimeErrorContains: "keyword format",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
			// libopenapi-validator v0.13.13 treats this format as an annotation.
			// kin-openapi v0.140.0 registers no email validator; OAS 3.0.3 leaves format policy to implementations.
		},
		{
			ID: "numeric-format-int32-boundary",
			Schema: `
      type: integer
      format: int32
`,
			Body:       []byte(`2147483647`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
		},
		{
			ID: "numeric-format-int32-overflow",
			Schema: `
      type: integer
      format: int32
`,
			Body:                    []byte(`2147483648`),
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "value should be between -2147483648 and 2147483647",
		},
		{
			ID: "numeric-format-int64-boundary",
			Schema: `
      type: integer
      format: int64
`,
			Body:       []byte(`9223372036854775807`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
		},
		{
			ID: "numeric-format-int64-overflow",
			Schema: `
      type: integer
      format: int64
`,
			Body:       []byte(`9223372036854775808`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
		},
		{
			ID: "numeric-format-float-double-unbounded",
			Schema: `
      type: number
      allOf:
        - {format: float}
        - {format: double}
`,
			Body:       []byte(`1e40`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
		},
		{
			ID: "numeric-format-unknown",
			Schema: `
      type: number
      format: vendor-number
`,
			Body:       []byte(`1.25`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
		},
		{
			ID: "numeric-format-mismatched",
			Schema: `
      type: integer
      format: double
`,
			Body:       []byte(`42`),
			Libopenapi: validatorBodyAccepted,
			Kinopenapi: validatorBodyAccepted,
			// OAS formats are open and tools may fall back to type alone.
		},
		{
			ID: "untyped-schema-null",
			Schema: `
      minLength: 0
`,
			Body:                    []byte(`null`),
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "Value is not nullable",
			// OAS 3.0.3 inherits the JSON Schema instance model, whose unconstrained schemas permit null.
			// kin-openapi v0.140.0 rejects null without nullable when type is omitted.
			// libopenapi-validator v0.13.13 accepts it.
		},
		{
			ID: "existing-typeless-mixed-keywords-null",
			Schema: `
      minLength: 1
      maxLength: 2
      minimum: -1
      maximum: 1
      minItems: 1
      maxItems: 2
      minProperties: 1
      maxProperties: 2
`,
			Body:                    []byte(`null`),
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "Value is not nullable",
			// This exact former integration fixture allows null under the JSON Schema instance model.
			// kin-openapi v0.140.0 rejects null when type is omitted; libopenapi-validator v0.13.13 accepts it.
		},
		{
			ID: "existing-zero-collection-bounds-null",
			Schema: `
      maxLength: 0
      maxItems: 0
      maxProperties: 0
`,
			Body:                    []byte(`null`),
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "Value is not nullable",
			// This exact former integration fixture permits null because collection keywords are inapplicable to it.
			// kin-openapi v0.140.0 rejects null when type is omitted; libopenapi-validator v0.13.13 accepts it.
		},
		{
			ID: "numeric-array-enum",
			Schema: `
      type: array
      items: {type: integer}
      enum: [[1]]
`,
			Body:                    []byte(`[1]`),
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "value is not one of the allowed values [[1]]",
			// Both JSON Schema and OAS compare array enum values structurally.
			// kin-openapi v0.140.0 compares decoded float64 values against integer-valued schema enum entries.
			// libopenapi-validator v0.13.13 accepts this exact member.
		},
		{
			ID: "empty-property-name",
			Schema: `
      type: object
      required: [""]
      properties:
        "": {type: string}
      additionalProperties: false
`,
			Body:                    []byte(`{"":""}`),
			Libopenapi:              validatorSetupRejected,
			LibopenapiErrorContains: "request schema failed to compile",
			Kinopenapi:              validatorBodyAccepted,
			// JSON object names may be empty.
			// libopenapi-validator v0.13.13 fails downstream schema compilation; kin-openapi v0.140.0 accepts it.
		},
		{
			ID: "nullable-allof-untyped-child",
			Schema: `
      type: string
      nullable: true
      allOf:
        - {minLength: 1}
`,
			Body:                    []byte(`null`),
			Libopenapi:              validatorBodyRejected,
			LibopenapiErrorContains: "'oneOf' failed, subschemas 0, 1 matched",
			Kinopenapi:              validatorBodyAccepted,
			// minLength is inapplicable to null, so this OAS 3.0.3 nullable schema should accept null.
			// libopenapi-validator v0.13.13 rejects it during schema validation while kin-openapi v0.140.0 accepts it.
		},
		{
			ID: "nullable-allof-contradictory-typed-children",
			Schema: `
      type: string
      nullable: true
      allOf:
        - {type: number}
        - {type: boolean}
`,
			Body:                 []byte(`null`),
			Runtime:              validatorBodyRejected,
			RuntimeErrorContains: "keyword type",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
			// The allOf children validate independently and both reject null, so the conjunction is empty.
			// libopenapi-validator v0.13.13 and kin-openapi v0.140.0 both incorrectly accept null.
		},
		{
			ID: "allof-nullable-children-nonnullable-parent",
			Schema: `
      type: boolean
      allOf:
        - {type: boolean, nullable: true}
        - {type: boolean, nullable: true}
`,
			Body:                    []byte(`null`),
			Runtime:                 validatorBodyRejected,
			RuntimeErrorContains:    "keyword type",
			Libopenapi:              validatorBodyRejected,
			LibopenapiErrorContains: "got null, want boolean",
			Kinopenapi:              validatorBodyAccepted,
			// The non-nullable parent type rejects null even though each allOf child independently permits it.
			// kin-openapi v0.140.0 incorrectly lets the nullable children make the parent nullable.
		},
		{
			ID: "reference-object-sibling-ignored",
			Schema: `
      $ref: '#/components/schemas/Text'
      minLength: 9
`,
			Components: `
components:
  schemas:
    Text: {type: string, minLength: 1}
`,
			Body:                    []byte(`"x"`),
			Libopenapi:              validatorSetupRejected,
			LibopenapiErrorContains: "schema node was not found in its root document",
			Kinopenapi:              validatorSetupRejected,
			KinopenapiErrorContains: "extra sibling fields: [minLength]",
			// OAS 3.0.3 says Reference Object siblings SHALL be ignored.
			// libopenapi-validator v0.13.13 fails schema compilation during the constructor probe.
			// kin-openapi v0.140.0 rejects the document during strict legacy-router setup.
		},
		{
			ID: "non-re2-negative-lookahead-pattern",
			Schema: `
      type: string
      pattern: '^(?!bad$)[a-z]+$'
      x-valid-examples: [good]
      x-invalid-examples: [bad]
`,
			Body:                    []byte(`"bad"`),
			Runtime:                 validatorSetupRejected,
			RuntimeErrorContains:    "unsupported RE2 pattern",
			Libopenapi:              validatorSetupRejected,
			LibopenapiErrorContains: "OpenAPI document is not valid according to the 3.0.3 specification",
			Kinopenapi:              validatorSetupRejected,
			KinopenapiErrorContains: "invalid or unsupported Perl syntax: `(?!`",
			// ECMAScript 5.1 permits negative lookahead, but Go RE2 does not; both pinned adapters reject it during setup.
		},
		{
			ID: "request-read-only-property",
			Schema: `
      type: object
      required: [id]
      properties:
        id: {type: string, readOnly: true}
      additionalProperties: false
`,
			Body:                    []byte(`{"id":"server"}`),
			Runtime:                 validatorSetupRejected,
			RuntimeErrorContains:    "readOnly",
			Libopenapi:              validatorBodyAccepted,
			Kinopenapi:              validatorBodyRejected,
			KinopenapiErrorContains: "readOnly property \"id\" in request",
			// OAS 3.0.3 says readOnly properties SHOULD NOT be sent in requests and removes their requiredness for requests.
			// libopenapi-validator v0.13.13 accepts this by default; kin-openapi v0.140.0 rejects it under VisitAsRequest.
		},
		{
			ID:                   "unsupported-one-of",
			Schema:               "      oneOf: [{type: string}]\n",
			Body:                 []byte(`"x"`),
			Runtime:              validatorSetupRejected,
			RuntimeErrorContains: "oneOf",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
		},
		{
			ID:                   "unsupported-any-of",
			Schema:               "      anyOf: [{type: string}]\n",
			Body:                 []byte(`"x"`),
			Runtime:              validatorSetupRejected,
			RuntimeErrorContains: "anyOf",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
		},
		{
			ID:                   "unsupported-not",
			Schema:               "      not: {type: number}\n",
			Body:                 []byte(`"x"`),
			Runtime:              validatorSetupRejected,
			RuntimeErrorContains: "not",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
		},
		{
			ID:                   "unsupported-write-only",
			Schema:               "      type: string\n      writeOnly: true\n",
			Body:                 []byte(`"x"`),
			Runtime:              validatorSetupRejected,
			RuntimeErrorContains: "writeOnly",
			Libopenapi:           validatorBodyAccepted,
			Kinopenapi:           validatorBodyAccepted,
		},
		{
			ID: "unsupported-discriminator",
			Schema: "      type: object\n" +
				"      discriminator: {propertyName: kind}\n",
			Body:                    []byte(`{}`),
			Runtime:                 validatorSetupRejected,
			RuntimeErrorContains:    "discriminator",
			Libopenapi:              validatorBodyRejected,
			LibopenapiErrorContains: "discriminator property 'kind' is missing",
			Kinopenapi:              validatorBodyAccepted,
		},
	}
}

// runValidatorCharacterizations checks one adapter's explicit expected phase for every fixed row.
func runValidatorCharacterizations(
	t *testing.T,
	factory validatorAdapterFactory,
	adapterName string,
) {
	t.Helper()

	for _, row := range validatorCharacterizations() {
		t.Run(row.ID, func(t *testing.T) {
			expected, errorContains := row.expectedValidatorOutcome(adapterName)
			runValidatorCharacterization(t, factory, expected, errorContains, row)
		})
	}
}

// expectedValidatorOutcome returns the exact phase and stable reason pinned for one adapter version.
func (row validatorCharacterization) expectedValidatorOutcome(adapterName string) (validatorPhase, string) {
	switch adapterName {
	case runtimeValidationName:
		return row.Runtime, row.RuntimeErrorContains
	case libopenapiValidatorName:
		return row.Libopenapi, row.LibopenapiErrorContains
	case kinopenapiValidatorName:
		return row.Kinopenapi, row.KinopenapiErrorContains
	default:
		return validatorPhase(255), ""
	}
}

// runValidatorCharacterization checks setup separately from a fixed-body verdict.
func runValidatorCharacterization(
	t *testing.T,
	factory validatorAdapterFactory,
	expected validatorPhase,
	errorContains string,
	row validatorCharacterization,
) {
	t.Helper()

	spec := append(requestBodySpec(row.Schema), row.Components...)

	adapter, setupErr := factory(spec)
	if expected == validatorSetupRejected {
		cleanupValidatorAdapter(adapter)

		if setupErr == nil {
			t.Fatal("expected setup error, but adapter was constructed")
		}

		assertCharacterizationError(t, setupErr, errorContains)

		return
	}

	if setupErr != nil {
		cleanupValidatorAdapter(adapter)

		t.Fatalf("unexpected setup error from adapter: %v", setupErr)
	}

	t.Cleanup(func() {
		cleanupValidatorAdapter(adapter)
	})

	validationErr := adapter.validator.Validate(row.Body)

	switch expected {
	case validatorBodyAccepted:
		if validationErr != nil {
			t.Fatalf("expected body acceptance from %s: %v", adapter.name, validationErr)
		}
	case validatorBodyRejected:
		if !isBodyRejection(validationErr) {
			t.Fatalf("expected body rejection from %s, got: %v", adapter.name, validationErr)
		}

		assertCharacterizationError(t, validationErr, errorContains)
	default:
		t.Fatalf("unknown expected phase %d", expected)
	}
}

// assertCharacterizationError prevents an unrelated failure from satisfying a pinned characterization.
func assertCharacterizationError(t *testing.T, err error, errorContains string) {
	t.Helper()

	if errorContains == "" {
		t.Fatal("rejected characterization has no pinned error reason")
	}

	if !strings.Contains(err.Error(), errorContains) {
		t.Fatalf("characterization error %q does not contain %q", err, errorContains)
	}
}
