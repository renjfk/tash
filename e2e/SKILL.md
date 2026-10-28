# Skill: run and extend the tash e2e suite

Use this when you need to verify tash end-to-end, debug a failing scenario, or
add coverage for a new feature. The suite drives the real binary against a real
model; there is no mock, so success means the whole loop works.

## TL;DR

```sh
make e2e                 # build + run all scenarios (needs the API key env)
./e2e/run.sh 10          # run one scenario by name substring
E2E_DEBUG=1 E2E_KEEP=1 ./e2e/run.sh 10   # keep HOME + dump state on failure
make e2e-docker          # run the whole suite in the reproducible container
```

Preconditions: `fish`, `tui-test` (for interactive scenarios), and the endpoint
config in the environment (`E2E_ENDPOINT`, `E2E_MODEL`, and the key named by
`E2E_API_KEY_ENV`, default `E2E_API_KEY`). There are no committed endpoint
defaults; scenarios skip until these are set.

## Mental model

Each scenario is a bash file that sources `e2e/lib.sh` and exits non-zero on
failure. It runs in a throwaway `HOME`; tash config, profile, conversation, and
usage all live under `$HOME/.config/tash`, and the fish hook under
`$HOME/.config/fish/conf.d/tash.fish`. Interactive scenarios use `tui-test` to
open a real fish PTY and assert on rendered text.

## Steps to run the suite

1. `make build` (or let `run.sh` build it).
2. Set `E2E_ENDPOINT`, `E2E_MODEL`, and the API key (e.g.
   `export E2E_API_KEY=...`, or point `E2E_API_KEY_ENV` at a different variable).
3. `./e2e/run.sh` (optionally filter by name substrings).
4. Read the summary. Non-zero exit means at least one scenario failed; the
   failing scenario's captured output is printed inline.

## Interpreting failures

- Assertion failures print the expected vs actual (truncated). Re-run the single
  scenario with `E2E_DEBUG=1 E2E_KEEP=1` to keep `$HOME` and dump
  `config.yaml`, `usage.json`, `profile.md`, and `conversation.jsonl`.
- For interactive failures, inspect the session:
  `tui-test --session <id> text --full` (the session id is in the scenario log).
  `tui-test` also writes traces per `e2e/tui-test.toml` (`traces/trace.md` on
  failure) — read `trace.md` before the HTML.
- Common causes: the model didn't follow the steering prompt (make the prompt
  more explicit), a token/param incompatibility (check `E2E_MAX_TOKENS_PARAM`
  and `E2E_REASONING_EFFORT`), or a stray terminal-probe leaking into fish (keep
  `TERM=dumb` / `NO_COLOR=1`).

## Debugging with tui-test directly

`tui-test` keeps named sessions across commands:

```sh
tui-test --session dbg open --shell fish --env HOME=/tmp/x --restart --wait-ready
tui-test --session dbg submit "list all files"
tui-test --session dbg expect text "ls" --timeout 30000
tui-test --session dbg text --full          # dump the screen
tui-test --session dbg close
```

Run `tui-test skill` for the full agent guide and `tui-test agent-context` for
the machine-readable command schema.

## Adding a scenario

1. Copy the nearest existing scenario in `e2e/scenarios/`.
2. `source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"`.
3. Call `e2e_setup` first; call `e2e_init` if you need the fish hook/profile.
4. Assert with `e2e_assert_*` (CLI) or `e2e_tt_expect*` (interactive), then
   `e2e_pass`.
5. Prefer markers the model must emit and file side effects over exact strings.

Helpers available in `e2e/lib.sh`: `e2e_setup`, `e2e_init`, `e2e_query`,
`e2e_write_config`, `e2e_history_add`, `e2e_assert_contains`, `e2e_assert_grep`,
`e2e_assert_file`, `e2e_wait_until`,
`e2e_query_was_called`, `e2e_tt_open_fish`, `e2e_tt_type_enter`, `e2e_tt_enter`,
`e2e_tt_expect`, `e2e_tt_expect_line`, `e2e_tt_expect_output`, `e2e_tt_close`.

Approval scenarios add `e2e_require_ntfy` (skips unless a reachable ntfy server
and token are configured), `e2e_approval_config` (appends an enabled approval
block to the config), and `e2e_ntfy_approve` (polls the request topic and taps
the timeframe button). See `18-approval-local` and `19-approval-ntfy`.

## Guardrails

- Do not commit the API key; it is supplied via the environment.
- Keep scenarios hermetic: use `e2e_setup`'s throwaway `HOME`; never touch the
  developer's real config.
- Interactive assertions must distinguish command output from the echoed input
  line (skip lines beginning with `>`), or they pass vacuously.