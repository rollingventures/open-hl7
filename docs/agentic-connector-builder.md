# Agentic Connector Builder

> Use a vision-capable Claude agent to look at an existing EMR/system's
> interface screens (and sample messages) and **synthesize a working connector
> on the fly** — instead of an integration engineer hand-reverse-engineering it.

This is open-hl7's differentiator. Traditional HL7 integration (Mirth et al.)
requires a human to read the vendor's interface-spec PDF, click through their
config screens, and hand-build channel + field maps. Most of that is *perception
+ mapping*, which a modern multimodal agent can do — and iterate on until its
output matches the target's real messages.

## Inputs (what the user provides)

Any mix of:
- **Screenshots** of the target system: the HL7/interface config screen, field
  layout / data-dictionary screens, the patient/encounter forms, sample message
  viewers, credential/endpoint screens.
- **Sample messages** (raw HL7, or a screenshot of one).
- Optional: a spec PDF, a few example records, the EMR's name/version.

## Output

A **`ConnectorSpec`** (see `internal/connectorgen`): a declarative,
human-reviewable connector definition — transport, MSH addressing, supported
message types, and a field map (canonical ⇄ HL7 path) — plus a confidence score
and notes. The hub's (config-driven) channel engine loads a `ConnectorSpec` and
runs it as a live channel. No bespoke code per integration.

## The agent loop

Built on the Claude **Messages API** (vision content blocks + tool use), default
model `claude-opus-4-8` for the perception/mapping reasoning, with a cheaper
`claude-sonnet-4-6` pass for bulk extraction.

```
1. PERCEIVE   vision: screenshots -> structured observations
              (system, version, message types, transport, field labels,
               MSH values, sample data, credential shape)
2. MAP        propose ConnectorSpec: transport + MSH routing + field map
              (their labels -> canonical -> HL7 segment/field path)
3. VALIDATE   generate a message from a synthetic canonical record using the
              proposed map; diff it against the target's real sample message.
              Self-correct and loop until it matches (or flag the gaps).
4. EMIT       write the ConnectorSpec; surface it for human approval.
5. DEPLOY     approved spec -> channel engine -> live channel.
```

Agent tools (function-calling): `record_observation`, `propose_field_map`,
`generate_sample_message` (calls the hub encoder), `diff_against_target`,
`request_more_screenshots` (ask the human for a missing screen),
`emit_connector_spec`. The loop is adversarial-by-design: VALIDATE must pass
before EMIT, so the agent cannot hand off a plausible-but-wrong map.

## Human-in-the-loop

The agent never auto-deploys. It produces a ConnectorSpec + a rendered sample
message + a confidence/gaps report; a human approves (or edits) before the
channel goes live. Low-confidence fields are flagged for review.

## Why this generalizes to "any Enterprise EMR"

The agent only ever emits a `ConnectorSpec` against the **neutral canonical
model**. The hub stays EMR-agnostic; each new system is a new spec, not new
code. OpenEMR (with its known `procedure_providers` + event seams) becomes a
high-confidence reference case the agent can be evaluated against.

## Build phases

- **CB-1** — `ConnectorSpec` schema + a config-driven channel engine that can
  run a spec (evolve the M1 hardcoded channel into spec-loading).
- **CB-2** — PERCEIVE + MAP from screenshots → draft spec (Messages API vision).
- **CB-3** — VALIDATE loop (generate ⇄ diff against sample) with self-correction.
- **CB-4** — approval UI + deploy; confidence scoring; gap reporting.

## Open questions

- Agent runtime: keep the loop in Go (Anthropic HTTP API directly) vs. a
  sidecar in TS/Python using the Claude Agent SDK (richer tool-loop ergonomics).
- How much to lean on sample-message diffing vs. spec-PDF parsing when both
  exist.
- Secrets: screenshots may contain PHI/credentials — must be scrubbed/redacted
  before sending to the API (a pre-processing redaction step).
