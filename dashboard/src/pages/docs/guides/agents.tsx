// Coding-agent guides: Claude Code, Codex CLI, Cursor, Cline / Roo Code, Continue, Aider.
import { A, C, Callout, Code, H1, H2, Ol, P, Ul, useDocs } from '../content'

export function ClaudeCode() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Point Anthropic's Claude Code CLI at the gateway with two environment variables.">Claude Code</H1>

      <H2 id="env">Environment variables</H2>
      <Code
        lang="bash"
        code={`export ANTHROPIC_BASE_URL=${base}
export ANTHROPIC_AUTH_TOKEN=YOUR_API_KEY
# optional: pin models
export ANTHROPIC_MODEL=claude-sonnet-4.5
export ANTHROPIC_SMALL_FAST_MODEL=claude-haiku-4.5

claude`}
      />
      <P>
        <C>ANTHROPIC_API_KEY</C> works as well; <C>ANTHROPIC_AUTH_TOKEN</C> is preferred because it skips the interactive login prompt. Do not
        add <C>/v1</C> to the base URL.
      </P>

      <H2 id="settings">Persist in ~/.claude/settings.json</H2>
      <Code
        lang="json"
        code={`{
  "env": {
    "ANTHROPIC_BASE_URL": "${base}",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
    "ANTHROPIC_MODEL": "claude-sonnet-4.5",
    "ANTHROPIC_SMALL_FAST_MODEL": "claude-haiku-4.5"
  }
}`}
      />
      <P>
        A project-level <C>.claude/settings.json</C> with the same <C>env</C> block overrides the global one, handy when only one repo should go
        through the gateway.
      </P>

      <H2 id="thinking">Thinking</H2>
      <P>
        Set <C>ANTHROPIC_MODEL=claude-sonnet-4.5-thinking</C>, or use <C>/model claude-sonnet-4.5-thinking</C> inside the session. Claude
        Code's own "ultrathink" keywords also send an Anthropic <C>thinking</C> block, which the gateway honours.
      </P>

      <H2 id="verify">Verify</H2>
      <Code lang="bash" code={`claude -p "Reply with the single word OK"`} />
      <P>
        Then open <A href="/usage">/usage</A> with your key: the request counter should have moved. Telemetry calls to{' '}
        <C>/api/event_logging/batch</C> are accepted and discarded by the gateway.
      </P>
    </>
  )
}

export function CodexCli() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="OpenAI's Codex CLI speaks the Responses API, which the gateway serves at /v1/responses.">Codex CLI</H1>

      <H2 id="config">~/.codex/config.toml</H2>
      <Code
        lang="toml"
        code={`model = "claude-sonnet-4.5"
model_provider = "hekato"

[model_providers.hekato]
name = "Hekato gateway"
base_url = "${base}/v1"
env_key = "HEKATO_API_KEY"
wire_api = "responses"`}
      />
      <Code
        lang="bash"
        code={`export HEKATO_API_KEY=YOUR_API_KEY
codex`}
      />

      <H2 id="chat-fallback">Chat Completions fallback</H2>
      <P>
        If a Codex version misbehaves with the Responses wire format, switch the provider to Chat Completions; everything else stays the same.
      </P>
      <Code
        lang="toml"
        code={`[model_providers.hekato]
name = "Hekato gateway (chat)"
base_url = "${base}/v1"
env_key = "HEKATO_API_KEY"
wire_api = "chat"`}
      />

      <H2 id="models">Models</H2>
      <Ul>
        <li>
          <C>model = "claude-sonnet-4.5"</C> for everyday work, <C>claude-opus-4.5</C> for hard problems, <C>claude-sonnet-4.5-thinking</C> for
          extended reasoning.
        </li>
        <li>
          <C>model = "auto"</C> lets the gateway pick per request (if the admin enabled Auto Routing).
        </li>
        <li>
          Codex's <C>model_reasoning_effort</C> setting is ignored by the gateway; use the <C>-thinking</C> suffix instead.
        </li>
      </Ul>

      <H2 id="verify">Verify</H2>
      <Code lang="bash" code={`codex exec "Reply with the single word OK"`} />
    </>
  )
}

export function Cursor() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Cursor can use an OpenAI-compatible base URL for its chat and composer features.">Cursor</H1>

      <H2 id="setup">Setup</H2>
      <Ol>
        <li>
          Open <strong>Cursor Settings → Models</strong>.
        </li>
        <li>
          Under <strong>OpenAI API Key</strong>, paste <C>YOUR_API_KEY</C>.
        </li>
        <li>
          Enable <strong>Override OpenAI Base URL</strong> and enter:
          <Code lang="text" code={`${base}/v1`} />
        </li>
        <li>
          Click <strong>Verify</strong>. Cursor sends a test request through the gateway.
        </li>
        <li>
          Click <strong>+ Add model</strong> and add the names you want to use, for example <C>claude-sonnet-4.5</C>, <C>claude-opus-4.5</C>,{' '}
          <C>claude-sonnet-4.5-thinking</C> and <C>auto</C>. Untick the built-in models so Cursor cannot fall back to them.
        </li>
      </Ol>

      <H2 id="caveats">Caveats</H2>
      <Callout kind="warning">
        Cursor always uses its own model IDs for some features (Tab completion, Apply, embeddings) regardless of the override, and those requests
        go to Cursor's servers, not the gateway. Only Chat / Composer with a custom model name is routed through your base URL.
      </Callout>
      <Ul>
        <li>
          If you pick a built-in name like <C>gpt-4o</C>, the gateway aliases it to <C>claude-sonnet-4.5</C>, so it still works.
        </li>
        <li>
          The override is global: turn it off to go back to Cursor's own models. Cursor may prompt you to disable it when using some features.
        </li>
        <li>Anthropic key / base URL overrides in Cursor are not used; keep everything under the OpenAI section.</li>
      </Ul>

      <H2 id="verify">Verify</H2>
      <P>
        Open Chat, select <C>claude-sonnet-4.5</C>, ask a question, then check <A href="/usage">/usage</A>.
      </P>
    </>
  )
}

export function ClineRoo() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Both VS Code agents accept a custom Anthropic or OpenAI-compatible endpoint.">Cline / Roo Code</H1>

      <H2 id="anthropic">Option A: Anthropic provider (recommended)</H2>
      <P>Native tool use, thinking blocks and prompt caching are all preserved on this path.</P>
      <Ul>
        <li>
          <strong>API Provider</strong>: Anthropic
        </li>
        <li>
          <strong>Anthropic API Key</strong>: <C>YOUR_API_KEY</C>
        </li>
        <li>
          <strong>Use custom base URL</strong>: enabled
        </li>
        <li>
          <strong>Base URL</strong>: <C>{base}</C> (no <C>/v1</C>)
        </li>
        <li>
          <strong>Model</strong>: <C>claude-sonnet-4.5</C>. If the model picker only lists dated IDs, pick any Sonnet 4.5 entry; the gateway
          normalizes dated names.
        </li>
      </Ul>

      <H2 id="openai">Option B: OpenAI Compatible provider</H2>
      <Ul>
        <li>
          <strong>API Provider</strong>: OpenAI Compatible
        </li>
        <li>
          <strong>Base URL</strong>: <C>{base}/v1</C>
        </li>
        <li>
          <strong>API Key</strong>: <C>YOUR_API_KEY</C>
        </li>
        <li>
          <strong>Model ID</strong>: <C>claude-sonnet-4.5</C> (or <C>auto</C>, <C>claude-sonnet-4.5-thinking</C>)
        </li>
        <li>
          Roo Code: leave <strong>Enable streaming</strong> on; set context window to 200000 if asked.
        </li>
      </Ul>

      <H2 id="thinking">Thinking</H2>
      <P>
        With the Anthropic provider, turning on "Extended thinking" in the agent settings sends a <C>thinking</C> block which the gateway
        honours. With the OpenAI-compatible provider, use the <C>-thinking</C> model name instead.
      </P>

      <H2 id="verify">Verify</H2>
      <P>
        Start a task such as "list the files in this workspace". The first response confirms the route; the request count on{' '}
        <A href="/usage">/usage</A> confirms the key.
      </P>
    </>
  )
}

export function Continue() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Continue reads models from ~/.continue/config.yaml. Either provider type works.">Continue</H1>

      <H2 id="anthropic">Anthropic provider</H2>
      <Code
        lang="yaml"
        code={`name: Local Assistant
version: 1.0.0
schema: v1

models:
  - name: Claude Sonnet 4.5 (Hekato)
    provider: anthropic
    model: claude-sonnet-4.5
    apiBase: ${base}
    apiKey: YOUR_API_KEY
    roles: [chat, edit, apply]

  - name: Claude Haiku 4.5 (Hekato)
    provider: anthropic
    model: claude-haiku-4.5
    apiBase: ${base}
    apiKey: YOUR_API_KEY
    roles: [autocomplete]`}
      />

      <H2 id="openai">OpenAI-compatible provider</H2>
      <Code
        lang="yaml"
        code={`models:
  - name: Claude Sonnet 4.5 (Hekato, OpenAI wire)
    provider: openai
    model: claude-sonnet-4.5
    apiBase: ${base}/v1
    apiKey: YOUR_API_KEY
    roles: [chat, edit, apply]`}
      />

      <H2 id="legacy">Legacy config.json</H2>
      <Code
        lang="json"
        code={`{
  "models": [
    {
      "title": "Claude Sonnet 4.5 (Hekato)",
      "provider": "anthropic",
      "model": "claude-sonnet-4.5",
      "apiBase": "${base}",
      "apiKey": "YOUR_API_KEY"
    }
  ]
}`}
      />

      <H2 id="verify">Verify</H2>
      <P>
        Reload the window, pick the model in the Continue sidebar, send "Reply with OK", then check <A href="/usage">/usage</A>.
      </P>
    </>
  )
}

export function Aider() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Aider uses LiteLLM under the hood; prefix the model with the provider you want to speak.">Aider</H1>

      <H2 id="openai">OpenAI-compatible</H2>
      <Code
        lang="bash"
        code={`export OPENAI_API_BASE=${base}/v1
export OPENAI_API_KEY=YOUR_API_KEY

aider --model openai/claude-sonnet-4.5`}
      />

      <H2 id="anthropic">Anthropic</H2>
      <Code
        lang="bash"
        code={`export ANTHROPIC_API_BASE=${base}
export ANTHROPIC_API_KEY=YOUR_API_KEY

aider --model anthropic/claude-sonnet-4.5`}
      />

      <H2 id="config">.aider.conf.yml</H2>
      <Code
        lang="yaml"
        code={`model: openai/claude-sonnet-4.5
weak-model: openai/claude-haiku-4.5
openai-api-base: ${base}/v1
openai-api-key: YOUR_API_KEY`}
      />
      <Callout>
        Aider warns "Unknown model" for names it has no metadata for. That is harmless; add a <C>.aider.model.metadata.json</C> with{' '}
        <C>max_input_tokens</C> if you want the warning gone.
      </Callout>

      <H2 id="verify">Verify</H2>
      <Code lang="bash" code={`aider --model openai/claude-sonnet-4.5 --message "Reply with the single word OK" --no-git`} />
    </>
  )
}
