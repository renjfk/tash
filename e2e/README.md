# tash end-to-end tests

Agent-driven, black-box tests that exercise the real `tash` binary against a
real OpenAI-compatible endpoint. Nothing is mocked: scenarios steer the model
with explicit prompts and assert on stable markers and observable side effects
(config, profile, conversation log, usage stats, and the fish command line).

## Why a real endpoint

tash's value is the whole loop: natural language in fish -> AI -> tool calls ->
a command placed in the command line buffer. Testing it against a stub would
only re-test the stub. Instead these tests run the real model and assert on
outcomes that are stable even as the model paraphrases.

## Prerequisites

- `fish` and `zellij` (zellij optional; only used for screen capture)
- [`tui-test`](https://github.com/microsoft/tui-test) for the interactive
  scenarios: `brew tap microsoft/tui-test https://github.com/microsoft/tui-test && brew install tui-test`
- `E2E_ENDPOINT`, `E2E_MODEL`, and an API key. There are no committed endpoint
  defaults, so scenarios skip until these are set:
  ```sh
  export E2E_ENDPOINT=https://your-endpoint/v1
  export E2E_MODEL=your-model
  export E2E_API_KEY=...        # or set E2E_API_KEY_ENV to your var's name
  ```

## Running

```sh
# host
make e2e                       # build tash, run every scenario
./e2e/run.sh 02 10             # only scenarios whose name contains "02" or "10"
E2E_KEEP=1 ./e2e/run.sh 10     # keep the throwaway HOME for inspection
E2E_DEBUG=1 ./e2e/run.sh 10    # dump config/profile/usage on failure

# container (reproducible: fish + zellij + tui-test + tash)
make e2e-docker
make e2e-docker ARGS="02 10"
```

Environment:

| Variable | Default | Purpose |
| --- | --- | --- |
| `E2E_ENDPOINT` | *(required)* | OpenAI-compatible base URL |
| `E2E_MODEL` | *(required)* | Deployment/model name |
| `E2E_API_KEY_ENV` | `E2E_API_KEY` | Name of the env var holding the key |
| `E2E_MAX_TOKENS_PARAM` | *(unset)* | `max_completion_tokens` for newer OpenAI models |
| `E2E_REASONING_EFFORT` | *(unset)* | e.g. `none`; some models require it for function tools |
| `E2E_TIMEOUT` | `30000` | Per-assertion timeout (ms) |

## Scenarios

| Scenario | Covers |
| --- | --- |
| `00-version` | CLI runs and reports a version |
| `01-init` | config + profile generation (via the model) + fish hook install |
| `02-query-command` | `tash query` returns a command on stdout |
| `03-query-chat` | conversational reply on stderr, no command |
| `04-usage` | `tash usage` counters (incl. cache column) and `--reset` |
| `05-reset-clear` | `reset` keeps memories; `clear` removes them |
| `06-caching` | prompt caching: a large stable prefix is served from cache |
| `10-fish-command` | fish hook: unknown command -> suggestion in the buffer -> runs |
| `11-fish-chat` | fish hook: a question returns chat text |
| `12-auto-intercept` | `tash tick` exit-7 path re-invokes a query for a failed command |
| `13-memory` | "remember ..." stores a durable memory entry |
| `14-multistep` | interactive approval of intermediate steps, final command queued |
| `15-screen-context` | screen-context plumbing degrades gracefully without Zellij |
| `16-screen-zellij` | **real Zellij capture**: marker on-screen is read into context (container-only) |
| `17-cancel` | Esc aborts an in-flight request and returns to the shell |

## How isolation works

Each scenario runs in a throwaway `HOME`, with `XDG_CONFIG_HOME` and
`XDG_DATA_HOME` unset and the `ZELLIJ*` variables cleared, so both tash
(`$HOME/.config/tash`) and fish (`$HOME/.config/fish`) resolve to it and tash
never tries to capture a real pane. tash `init` installs the fish hook there,
and the interactive session starts fish with that `HOME`. The fish hook points
at the absolute path of the binary under test (`E2E_TASH_BIN`, default
`bin/tash`).

## Design notes

- **Steering over mocking.** Prompts ask for explicit markers ("print exactly
  `E2EFISHMARKER`") and assertions check for those markers or file side effects.
- **Output vs input.** Fish echoes the typed line, so output assertions ignore
  lines starting with the prompt (`>`).
- **Color probes.** tash/lipgloss may query the terminal background colour; the
  async reply can leak into the shell as typed input. Sessions set `TERM=dumb`
  and `NO_COLOR=1` to avoid it.
- **Caching.** `06-caching` writes a large stable profile, runs a query, resets
  usage, then runs a second query and asserts the second request served >=80% of
  its prompt tokens from the provider cache. It doubles as a regression guard for
  keeping volatile data (the current time) out of the system prompt; the Go unit
  test is the primary guard.

## Known gaps / TODO

- Legacy JSONL fallback (needs an endpoint that rejects `tools`).

## Container-only scenario

`16-screen-zellij` drives a real Zellij session and is **container-only** because
driving Zellij from a host that already has a live session is unsafe. It runs
only when `E2E_ALLOW_ZELLIJ=1` (set by the Docker image) **and** a container is
detected (`/.dockerenv` or `/run/.containerenv`); otherwise it skips. It also
isolates Zellij via `ZELLIJ_SOCKET_DIR` so it can never reach a host session.

## Adding a scenario

Create `e2e/scenarios/NN-name.sh`, `source` `../lib.sh`, call `e2e_setup`, and
end with `e2e_pass "<what passed>"`. Failures go through `e2e_fail` (which dumps
state when `E2E_DEBUG=1`). Interactive scenarios use
`e2e_tt_open_fish` / `e2e_tt_type_enter` / `e2e_tt_expect*` / `e2e_tt_close`.