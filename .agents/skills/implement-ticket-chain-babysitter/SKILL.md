---
name: implement-ticket-chain-babysitter
description: Babysits the repository's implementation-ticket chain in a persistent Paseo terminal, monitors progress, diagnoses stopped or stalled runs, and resumes from the smallest safe boundary. Use when running or recovering the bundled ticket-chain workflow.
---

# Implement Ticket Chain Babysitter

GOAL

Keep the implementation-ticket train rolling until every ticket in [`issues.txt`](issues.txt) is handled to completion. You are the babysitter: start the workflow, watch it, and make the smallest safe intervention needed to keep it moving. Be hands-off while normal work is running; do not continuously inspect GitHub or interfere with healthy agents. Diagnose only after the terminal command exits or after the same ticket has remained active across four consecutive 20-minute checks. Do not merely report the first failure. Do not create subagents.

INPUTS

Resolve every bundled path from this skill directory; never depend on the caller's current directory. Set `SKILL_DIR` to the absolute parent directory of this loaded `SKILL.md`, using the skill location supplied by Pi. Then initialize paths exactly as follows:

```bash
ROOT=$(git -C "$SKILL_DIR" rev-parse --show-toplevel)
SCRIPT="$SKILL_DIR/implement-ticket-chain.sh"
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

Use one named Paseo terminal so the workflow survives your individual turns and all attempts remain in one scrollback. Do not manage an OS PID and do not write a polling shell script.

First inspect existing terminals:

```bash
paseo terminal ls --all --cwd "$ROOT" --json
```

If `implement-ticket-chain-babysitter` already exists, read its full scrollback before doing anything:

```bash
paseo terminal capture --scrollback implement-ticket-chain-babysitter
```

If that scrollback shows a chain attempt with no matching exit marker yet, attach to it and begin the 20-minute monitoring cycle. Do not send another workflow command: it could be queued behind the active one and run unexpectedly later. If the prior attempt ended, reuse the terminal. Create it only when it does not exist:

```bash
paseo --quiet terminal create --cwd "$ROOT" --name implement-ticket-chain-babysitter
```

Only when no chain command is active, read the wayfinder URL, build a uniquely marked command, and send it to the terminal. Run these as one direct command, recording the printed attempt marker:

```bash
WAYFINDER=$(awk 'NF && $1 !~ /^#/ {print}' "$WAYFINDER_FILE")
ATTEMPT="$(date +%s)"
printf -v RUN '%q %q %q; rc=$?; printf "\\n__TICKET_CHAIN_EXIT_%s__:%s\\n" "$rc"' \
  "$SCRIPT" "$ISSUES" "$WAYFINDER" "$ATTEMPT" '%s'
paseo terminal send-keys --literal implement-ticket-chain-babysitter "$RUN"
paseo terminal send-keys implement-ticket-chain-babysitter Enter
printf 'attempt marker: __TICKET_CHAIN_EXIT_%s__\n' "$ATTEMPT"
```

After every launch or recovery attempt, wait with a direct command:

```bash
sleep 1200
```

Then capture the terminal:

```bash
paseo terminal capture --scrollback implement-ticket-chain-babysitter
```

Do not combine the sleep and capture into a polling script. Repeat every 20 minutes while work is actively progressing. Look for the current attempt's exact exit marker; an older marker does not mean the current attempt ended.

ARBITRATION AND KEEPING IT MOVING

Normally, each 20-minute check is terminal-only. Capture the scrollback, identify the active ticket, and otherwise leave the workflow alone. Do not run `gh`, inspect repository state, or message agents merely because work is taking time.

Track in your reasoning how many consecutive 20-minute checks have shown the same active ticket. Movement to another ticket resets that count. Only when the command exits or the same ticket remains active for four consecutive checks (about 80 minutes) may you inspect read-only `git`, `gh`, and `paseo inspect` evidence and judge whether intervention is needed.

Progress includes a new commit or push, a new PR, an advanced workflow stage, a completed review/check/fix cycle, a merge, a closed ticket, or movement to the next ticket. Normal long review/manager work is not automatically a stall.

If there is no exit marker:
- Before the fourth consecutive check on the same ticket, always sleep another 20 minutes without further investigation.
- At the fourth check, inspect GitHub, Git, and relevant Paseo agents. Make a judgment: healthy work may simply need more time, in which case wait another 20 minutes. Intervene only for a concrete blocker or genuine lack of progress.
- You may message an existing agent directly when, after that investigation, it is the smallest safe intervention. Do not edit code yourself.
- Do not kill or restart healthy long-running work.

If the command exits nonzero:
1. Read the complete relevant scrollback, especially the structured failure footer.
2. Inspect current Git, GitHub, and Paseo state.
3. Determine the actual failed boundary instead of blindly repeating the command.
4. Choose the smallest resume step and a targeted babysitter prompt.
5. Relaunch in the same terminal with a new unique exit marker, then return to the 20-minute checks.

Resume-step meanings:
- `ticket`: start the selected ticket from the beginning.
- `implementer`: continue or replace the implementer, then proceed normally.
- `after-implementer`: the implementation PR handoff is already good; start normal manager setup.
- `manager`: continue or replace a manager whose lifecycle should finish the ticket.

When a prompt is required, write only the situation-specific instructions to a temporary file such as `/tmp/implement-ticket-chain-resume.txt`. Writing prompt files is your only allowed file modification.

Examples, with actual values substituted before sending the command to the same Paseo terminal:

```bash
# Restart selected ticket from its beginning
"$SCRIPT" --resume-ticket "$TICKET" --resume-from ticket \
  "$ISSUES" "$WAYFINDER"

# Continue a healthy stopped implementer
"$SCRIPT" --resume-ticket "$TICKET" --resume-from implementer \
  --resume-agent "$AGENT_ID" --resume-prompt-file "$PROMPT_FILE" \
  "$ISSUES" "$WAYFINDER"

# Replace a broken implementer by omitting --resume-agent
"$SCRIPT" --resume-ticket "$TICKET" --resume-from implementer \
  --resume-prompt-file "$PROMPT_FILE" "$ISSUES" "$WAYFINDER"

# Skip a brittle/irrelevant implementation check after verifying the PR handoff yourself
"$SCRIPT" --resume-ticket "$TICKET" --resume-from after-implementer \
  --resume-prompt-file "$PROMPT_FILE" "$ISSUES" "$WAYFINDER"

# Continue a healthy stopped manager; omit --resume-agent to replace a broken manager
"$SCRIPT" --resume-ticket "$TICKET" --resume-from manager \
  --resume-agent "$AGENT_ID" --resume-prompt-file "$PROMPT_FILE" \
  "$ISSUES" "$WAYFINDER"
```

Wrap every resumed command with a fresh `__TICKET_CHAIN_EXIT_<attempt>__:<status>` marker exactly as for the initial command. Do not start it in a second terminal.

Prefer the script's resume interface, but use judgment rather than ceremony. Only after the command exits or the four-check threshold is reached may you directly message an existing agent when clearly simpler. Likewise, use `gh` operationally only after one of those triggers and only for an obvious safe end-of-ticket operation—for example, merging an already-ready PR and closing its satisfied leaf—then restart at the next ticket. Never invent or adjudicate implementation findings. Never merge with pending or failed required checks, unresolved accepted work, or uncertain state. Never edit code or repository files.

NO-PROGRESS RULE

Track this only in your own reasoning; do not create a counter file or add logic to the workflow script.

- Before retrying, explain to yourself why the prior attempt got stuck and change the prompt or resume point accordingly. Never repeat blindly.
- Allow at most two recovery attempts at the same state when neither attempt makes observable progress.
- Any observable progress resets the no-progress count to zero, even if a later failure occurs.
- After two no-progress recovery attempts, stop. Report the attempted interventions, diagnosis, relevant terminal evidence, current ticket/stage/agent IDs, and what needs user judgment.

COMPLETION

An exit status of zero is not enough by itself. Verify from GitHub that every ticket listed in the skill-local [`issues.txt`](issues.txt) is completed and that the workflow reached the end. Report completion and the Paseo terminal name/ID. Leave the terminal alive with its full scrollback on success or escalation; never kill it.
