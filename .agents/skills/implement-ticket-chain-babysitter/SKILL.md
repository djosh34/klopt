---
name: implement-ticket-chain-babysitter
description: Babysits the repository's implementation-ticket chain in a named transient systemd user service, monitors progress, diagnoses stopped or stalled runs, and resumes from the smallest safe boundary. Use when running or recovering the bundled ticket-chain workflow.
---

# Implement Ticket Chain Babysitter

GOAL

Keep the implementation-ticket train rolling until every ticket in [`issues.txt`](issues.txt) is handled to completion. You are the babysitter: start the workflow, watch it, and make the smallest safe intervention needed to keep it moving. Be hands-off while normal work is running; do not continuously inspect GitHub or interfere with healthy agents. Diagnose only after the systemd service exits or after the same ticket has remained active across four consecutive 20-minute checks. Do not merely report the first failure. Do not create subagents.

INPUTS

Resolve every bundled path from this skill directory; never depend on the caller's current directory. Set `SKILL_DIR` to the absolute parent directory of this loaded `SKILL.md`, using the skill location supplied by Pi. Then initialize paths exactly as follows:

```bash
ROOT=$(git -C "$SKILL_DIR" rev-parse --show-toplevel)
START_SCRIPT="$SKILL_DIR/start.sh"
STATUS_SCRIPT="$SKILL_DIR/status.sh"
LOGS_SCRIPT="$SKILL_DIR/logs.sh"
STOP_SCRIPT="$SKILL_DIR/stop.sh"
ISSUES="$SKILL_DIR/issues.txt"
WAYFINDER_FILE="$SKILL_DIR/wayfinder.txt"
```

Before running the workflow, users must inspect and edit the bundled [`issues.txt`](issues.txt) and [`wayfinder.txt`](wayfinder.txt) for the intended chain. `issues.txt` must contain the ordered implementation-ticket queue: one full GitHub issue URL per nonblank, non-comment line, for example:

```text
https://github.com/OWNER/REPO/issues/101
https://github.com/OWNER/REPO/issues/102
```

`wayfinder.txt` must contain exactly one nonblank, non-comment full GitHub issue URL. Any bundled comment-only example file is intentionally not runnable input until edited. Validate both files before starting. If either file is missing or malformed, stop and ask the user. Never guess or rewrite inputs.

START AND WATCH THE WORKFLOW

The workflow runs in the fixed named transient systemd user service `implement-ticket-chain-worker.service`. Always control and inspect it through the bundled scripts. Never invoke `systemd-run`, `systemctl`, or `journalctl` directly. Do not manage an OS PID and do not write a polling shell script. `start.sh` starts the service in `ROOT`, preserves the caller's Paseo attachment, and passes every argument unchanged to `worker.sh`.

First inspect whether the service is already running:

```bash
"$STATUS_SCRIPT"
```

If it is active, do not launch another workflow. Begin the 20-minute monitoring cycle. If it is inactive, failed, or no longer loaded, inspect its latest logs before deciding whether to start or recover it:

```bash
"$LOGS_SCRIPT" --no-pager -n 20
```

`logs.sh` passes every argument unchanged to `journalctl` while fixing the service filter. By default, request only the last 20 lines as shown above. Request more lines or a wider time range only when the last 20 lines are insufficient for diagnosis.

Only when no workflow service is active, read the wayfinder URL and launch the workflow through `start.sh`:

```bash
WAYFINDER=$(awk 'NF && $1 !~ /^#/ {print}' "$WAYFINDER_FILE")
"$START_SCRIPT" "$ISSUES" "$WAYFINDER"
```

After every launch or recovery attempt, wait with a direct command:

```bash
sleep 1200
```

At every 20-minute check, always inspect service status first, then read the last 20 log lines:

```bash
"$STATUS_SCRIPT"
"$LOGS_SCRIPT" --no-pager -n 20
```

Do not combine the sleep, status, or logs commands into a polling script. Repeat every 20 minutes while work is actively progressing.

ARBITRATION AND KEEPING IT MOVING

Normally, each 20-minute check is systemd-only. Use `status.sh`, read only the last 20 lines through `logs.sh`, identify the active ticket, and otherwise leave the workflow alone. Do not run `gh`, inspect repository state, or message agents merely because work is taking time.

Track in your reasoning how many consecutive 20-minute checks have shown the same active ticket. Movement to another ticket resets that count. Only when the service exits or the same ticket remains active for four consecutive checks (about 80 minutes) may you inspect read-only `git`, `gh`, and `paseo inspect` evidence and judge whether intervention is needed.

Progress includes a new commit or push, a new PR, an advanced workflow stage, a completed review/check/fix cycle, a merge, a closed ticket, or movement to the next ticket. Normal long review/manager work is not automatically a stall.

If the service is still active:
- Before the fourth consecutive check on the same ticket, always sleep another 20 minutes without further investigation.
- At the fourth check, inspect GitHub, Git, and relevant Paseo agents. Make a judgment: healthy work may simply need more time, in which case wait another 20 minutes. Intervene only for a concrete blocker or genuine lack of progress.
- You may message an existing agent directly when, after that investigation, it is the smallest safe intervention. Do not edit code yourself.
- Do not stop or restart healthy long-running work. When stopping is actually required, always use `"$STOP_SCRIPT"`.

If the service exits nonzero:
1. Use `logs.sh` to read the complete relevant journal range, especially the structured failure footer. Start with the last 20 lines and request more only as needed.
2. Inspect current Git, GitHub, and Paseo state.
3. Determine the actual failed boundary instead of blindly repeating the workflow.
4. Choose the smallest resume step and a targeted babysitter prompt.
5. Relaunch through `start.sh`, then return to the 20-minute checks.

Resume-step meanings:
- `ticket`: start the selected ticket from the beginning.
- `implementer`: continue or replace the implementer, then proceed normally.
- `after-implementer`: the implementation PR handoff is already good; start normal manager setup.
- `manager`: continue or replace a manager whose lifecycle should finish the ticket.

When a prompt is required, write only the situation-specific instructions to a temporary file such as `/tmp/implement-ticket-chain-resume.txt`. Writing prompt files is your only allowed file modification.

Examples, with actual values substituted before starting the systemd service:

```bash
# Restart selected ticket from its beginning
"$START_SCRIPT" --resume-ticket "$TICKET" --resume-from ticket \
  "$ISSUES" "$WAYFINDER"

# Continue a healthy stopped implementer
"$START_SCRIPT" --resume-ticket "$TICKET" --resume-from implementer \
  --resume-agent "$AGENT_ID" --resume-prompt-file "$PROMPT_FILE" \
  "$ISSUES" "$WAYFINDER"

# Replace a broken implementer by omitting --resume-agent
"$START_SCRIPT" --resume-ticket "$TICKET" --resume-from implementer \
  --resume-prompt-file "$PROMPT_FILE" "$ISSUES" "$WAYFINDER"

# Skip a brittle/irrelevant implementation check after verifying the PR handoff yourself
"$START_SCRIPT" --resume-ticket "$TICKET" --resume-from after-implementer \
  --resume-prompt-file "$PROMPT_FILE" "$ISSUES" "$WAYFINDER"

# Continue a healthy stopped manager; omit --resume-agent to replace a broken manager
"$START_SCRIPT" --resume-ticket "$TICKET" --resume-from manager \
  --resume-agent "$AGENT_ID" --resume-prompt-file "$PROMPT_FILE" \
  "$ISSUES" "$WAYFINDER"
```

Never call `worker.sh` directly. Every initial or resumed attempt must go through `start.sh`.

Prefer the worker's resume interface through `start.sh`, but use judgment rather than ceremony. Only after the service exits or the four-check threshold is reached may you directly message an existing agent when clearly simpler. Likewise, use `gh` operationally only after one of those triggers and only for an obvious safe end-of-ticket operation—for example, merging an already-ready PR and closing its satisfied leaf—then restart at the next ticket. Never invent or adjudicate implementation findings. Never merge with pending or failed required checks, unresolved accepted work, or uncertain state. Never edit code or repository files.

NO-PROGRESS RULE

Track this only in your own reasoning; do not create a counter file or add logic to the workflow script.

- Before retrying, explain to yourself why the prior attempt got stuck and change the prompt or resume point accordingly. Never repeat blindly.
- Allow at most two recovery attempts at the same state when neither attempt makes observable progress.
- Any observable progress resets the no-progress count to zero, even if a later failure occurs.
- After two no-progress recovery attempts, stop. Report the attempted interventions, diagnosis, relevant systemd journal evidence, current ticket/stage/agent IDs, and what needs user judgment.

COMPLETION

An exit status of zero is not enough by itself. Verify from GitHub that every ticket listed in the skill-local [`issues.txt`](issues.txt) is completed and that the workflow reached the end. Report completion and the systemd service name `implement-ticket-chain-worker.service`. Preserve its journal on success or escalation. Stop the service through `stop.sh` only when intervention actually requires it; never invoke systemd directly.
