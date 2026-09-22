// Core reference pages: getting started, endpoints, models, limits, troubleshooting.
/* oxlint-disable react/jsx-key -- static table cells; T() keys rows and cells by index */
import { A, C, Callout, Code, H1, H2, H3, P, T, Ul, useDocs } from './content'

export function GettingStarted() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Hekato-Go is an AI gateway. It exposes Anthropic- and OpenAI-compatible APIs in front of pools of Kiro, CodeBuddy and Grok accounts, with load balancing, per-key limits and smart routing.">
        Getting started
      </H1>

      <H2 id="what">What this gateway does</H2>
      <P>
        You talk to one host using the API shape you already know (Anthropic Messages, OpenAI Chat Completions or OpenAI Responses). The gateway
        picks an upstream account, translates the request, streams the answer back and tracks your usage. Any tool that can point at a custom
        Anthropic or OpenAI base URL works unchanged.
      </P>

      <H2 id="base-url">Base URL</H2>
      <Code lang="text" code={base} />
      <P>
        Anthropic-style clients use the base URL as-is (they append <C>/v1/messages</C>). OpenAI-style clients usually want <C>{base}/v1</C>.
      </P>

      <H2 id="api-key">Get an API key</H2>
      <P>
        Keys are created by the gateway administrator under <A href="/admin">Admin</A> → API Keys. Each key can carry its own token / credit
        quota, requests-per-minute and concurrency limits. Send it as <C>Authorization: Bearer YOUR_API_KEY</C> or <C>X-Api-Key: YOUR_API_KEY</C>.
      </P>
      <Callout>
        Paste your key into the field at the top of this page and every snippet below is rewritten with it, ready to copy.
      </Callout>

      <H2 id="anthropic-30s">30 seconds: Anthropic Messages</H2>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "Say hello in one sentence."}]
  }'`}
      />

      <H2 id="openai-30s">30 seconds: OpenAI Chat Completions</H2>
      <Code
        lang="bash"
        code={`curl ${base}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [{"role": "user", "content": "Say hello in one sentence."}]
  }'`}
      />

      <H2 id="check-usage">Check your usage</H2>
      <P>
        Open <A href="/usage">{base}/usage</A>, paste your key, and see quota, limits and current consumption. The same data is available as
        JSON from <C>GET /v1/usage</C> (see <A href="/docs/limits">Limits</A>).
      </P>

      <H2 id="next">Next steps</H2>
      <Ul>
        <li>
          <A href="/docs/endpoints">Endpoints</A> — every route, streaming, error format.
        </li>
        <li>
          <A href="/docs/models">Models</A> — model names, thinking mode, the <C>auto</C> model.
        </li>
        <li>Tool guides in the sidebar — Claude Code, Codex, Cursor, SDKs and more.</li>
      </Ul>
    </>
  )
}

export function Endpoints() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Every public route served by the gateway, how to authenticate, and how errors look in each dialect.">Endpoints</H1>

      <H2 id="overview">Overview</H2>
      <T
        head={['Method', 'Path', 'Purpose', 'Auth']}
        rows={[
          ['POST', <C>/v1/messages</C>, 'Anthropic Messages API (streaming and non-streaming)', 'API key'],
          ['POST', <C>/v1/messages/count_tokens</C>, 'Estimate input tokens for an Anthropic request', 'API key'],
          ['POST', <C>/v1/chat/completions</C>, 'OpenAI Chat Completions API', 'API key'],
          ['POST', <C>/v1/responses</C>, 'OpenAI Responses API', 'API key'],
          ['GET', <C>/v1/models</C>, 'List available model IDs', 'none'],
          ['GET', <C>/v1/usage</C>, 'Quota and usage of the calling key (JSON)', 'API key'],
          ['GET', <C>/health</C>, 'Liveness: status, version, uptime', 'none'],
        ]}
      />
      <P>
        The <C>/v1</C> prefix is optional: <C>/messages</C>, <C>/chat/completions</C> and <C>/responses</C> resolve to the same handlers.
        <C>/anthropic/v1/messages</C> is accepted too for clients that hard-code that layout.
      </P>

      <H2 id="auth">Authentication</H2>
      <P>Send the key in either header; both dialects accept both forms.</P>
      <Code
        lang="http"
        code={`Authorization: Bearer YOUR_API_KEY
# or
X-Api-Key: YOUR_API_KEY`}
      />
      <Callout kind="warning">
        Keys are checked on every request. A missing, unknown or disabled key returns <C>401</C>; a key over its quota or rate limit returns{' '}
        <C>429</C> with a <C>Retry-After</C> header.
      </Callout>

      <H2 id="messages">POST /v1/messages</H2>
      <P>
        Standard Anthropic Messages request body: <C>model</C>, <C>max_tokens</C>, <C>messages</C>, optional <C>system</C>, <C>tools</C>,
        <C>thinking</C>, <C>stream</C>. Responses use the Anthropic shape (<C>content</C> blocks, <C>stop_reason</C>, <C>usage</C>).
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":512,"system":"Be brief.","messages":[{"role":"user","content":"What is an AI gateway?"}]}'`}
      />

      <H2 id="count-tokens">POST /v1/messages/count_tokens</H2>
      <P>
        Same body as <C>/v1/messages</C> minus <C>max_tokens</C>. Returns <C>{'{"input_tokens": N}'}</C>. The count is a local estimate, useful
        for budgeting before you send the real request.
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages/count_tokens \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"How many tokens is this?"}]}'`}
      />

      <H2 id="chat-completions">POST /v1/chat/completions</H2>
      <P>
        OpenAI Chat Completions request: <C>model</C>, <C>messages</C>, optional <C>tools</C>, <C>stream</C>, <C>max_tokens</C> /
        <C>max_completion_tokens</C>, <C>temperature</C>. Image parts (<C>image_url</C>) and tool calls are translated to the upstream format.
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"system","content":"Be brief."},{"role":"user","content":"What is an AI gateway?"}]}'`}
      />

      <H2 id="responses">POST /v1/responses</H2>
      <P>
        OpenAI Responses API, used by Codex CLI and newer SDK code paths. Accepts <C>input</C> as a string or an item list, <C>instructions</C>,
        <C>tools</C>, <C>previous_response_id</C> and <C>stream</C>. Conversation state referenced by <C>previous_response_id</C> is kept in
        memory on the gateway.
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/responses \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","input":"What is an AI gateway?"}'`}
      />

      <H2 id="models-endpoint">GET /v1/models</H2>
      <P>
        Lists the models the account pool currently advertises, plus the aliases <C>auto</C>, <C>gpt-4o</C> and <C>gpt-4</C>, and a{' '}
        <C>-thinking</C> variant for each Claude model. No key is needed.
      </P>
      <Code lang="bash" code={`curl ${base}/v1/models`} />

      <H2 id="usage-endpoint">GET /v1/usage</H2>
      <P>Returns the calling key's name, masked value, limits and consumption. Powers the public /usage page.</P>
      <Code lang="bash" code={`curl ${base}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`} />

      <H2 id="health">GET /health</H2>
      <Code lang="bash" code={`curl ${base}/health\n# {"status":"ok","version":"...","uptime":12345}`} />

      <H2 id="streaming">Streaming (SSE)</H2>
      <P>
        Set <C>"stream": true</C> in any chat endpoint. The response is <C>text/event-stream</C> using the native event format of the dialect
        you called:
      </P>
      <Ul>
        <li>
          Anthropic: <C>message_start</C>, <C>content_block_start</C>, <C>content_block_delta</C>, <C>message_delta</C>, <C>message_stop</C>.
        </li>
        <li>
          OpenAI Chat: <C>data: {'{...chat.completion.chunk...}'}</C> lines terminated by <C>data: [DONE]</C>.
        </li>
        <li>
          OpenAI Responses: <C>response.created</C>, <C>response.output_text.delta</C>, <C>response.completed</C>.
        </li>
      </Ul>
      <Code
        lang="bash"
        code={`curl -N ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","stream":true,"messages":[{"role":"user","content":"Count to five."}]}'`}
      />
      <Callout>
        If you sit behind nginx, Cloudflare or another reverse proxy, disable response buffering for these paths, otherwise tokens arrive in one
        burst at the end. See <A href="/docs/troubleshooting#streaming">Troubleshooting</A>.
      </Callout>

      <H2 id="errors">Error format</H2>
      <P>Errors follow the dialect of the endpoint you called.</P>
      <H3 id="errors-anthropic">Anthropic endpoints</H3>
      <Code lang="json" code={`{"type":"error","error":{"type":"authentication_error","message":"Invalid or missing API key"}}`} />
      <H3 id="errors-openai">OpenAI endpoints</H3>
      <Code lang="json" code={`{"error":{"type":"authentication_error","message":"Invalid or missing API key"}}`} />

      <H2 id="status-codes">Status codes</H2>
      <T
        head={['Status', 'Type', 'Meaning']}
        rows={[
          ['400', <C>invalid_request_error</C>, 'Body is not valid JSON or fails validation'],
          ['401', <C>authentication_error</C>, 'Key missing, unknown or disabled'],
          ['429', <C>rate_limit_error</C>, 'Token / credit quota, RPM or concurrency limit hit. Retry-After header is set.'],
          ['503', <C>api_error</C> , 'No available accounts in the pool for this model'],
        ]}
      />
      <P>
        On <C>429</C> the gateway sets <C>Retry-After: 1</C> (seconds). Back off at least that long; for quota exhaustion, ask your admin to raise
        the limit or reset usage.
      </P>
    </>
  )
}

export function Models() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="How model names are resolved, how to turn on extended thinking, and how the auto model routes for you.">Models</H1>

      <H2 id="names">Model names</H2>
      <P>
        Use Anthropic-style IDs. The canonical names are <C>claude-sonnet-4.5</C>, <C>claude-opus-4.5</C> and <C>claude-haiku-4.5</C>;
        whatever the pool currently advertises is listed by <C>GET /v1/models</C>.
      </P>
      <T
        head={['You send', 'Resolves to', 'Note']}
        rows={[
          [<C>claude-sonnet-4.5</C>, <C>claude-sonnet-4.5</C>, 'Canonical form'],
          [<C>claude-sonnet-4-5</C>, <C>claude-sonnet-4.5</C>, 'Dash form is normalized'],
          [<C>claude-sonnet-4-5-20250929</C>, <C>claude-sonnet-4.5</C>, 'Dated suffixes are stripped'],
          [<C>gpt-4o</C>, <C>claude-sonnet-4.5</C>, 'OpenAI alias for tools that hard-code GPT names'],
          [<C>gpt-4</C>, <C>claude-sonnet-4.5</C>, 'Same alias'],
          [<C>auto</C>, 'chosen per request', 'See Auto routing below'],
        ]}
      />
      <Code lang="bash" code={`curl ${base}/v1/models | jq '.data[].id'`} />

      <H2 id="thinking">Thinking mode</H2>
      <P>Two ways to enable extended thinking, both work on every endpoint:</P>
      <H3 id="thinking-suffix">1. Model suffix</H3>
      <P>
        Append <C>-thinking</C> (the default suffix; the admin can change it under Settings → Thinking Mode). This is the only option for
        OpenAI-style clients.
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5-thinking","messages":[{"role":"user","content":"Prove that sqrt(2) is irrational."}]}'`}
      />
      <H3 id="thinking-block">2. Anthropic thinking block</H3>
      <P>
        On <C>/v1/messages</C>, a top-level <C>thinking</C> object enables it automatically: <C>{'{"type":"enabled","budget_tokens":2048}'}</C>{' '}
        or <C>{'{"type":"adaptive"}'}</C>.
      </P>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "max_tokens": 4096,
    "thinking": {"type": "enabled", "budget_tokens": 2048},
    "messages": [{"role": "user", "content": "Prove that sqrt(2) is irrational."}]
  }'`}
      />
      <Callout>
        How thinking is surfaced (Anthropic <C>thinking</C> content blocks, or inline <C>{'<thinking>'}</C> tags in text) is a gateway-wide
        setting chosen by the admin.
      </Callout>

      <H2 id="auto">The auto model</H2>
      <P>
        Send <C>"model": "auto"</C> and the gateway classifies the request (input size, tool count, turns, images, thinking) into a fast /
        balanced / strong tier, then a learning bandit picks the best (account, model) pair inside that tier using observed success rate, latency
        and remaining quota.
      </P>
      <P>The decision is reported in two response headers:</P>
      <T
        head={['Header', 'Value']}
        rows={[
          [<C>X-Hekato-Routed-Model</C>, 'The concrete model that served the request, e.g. claude-haiku-4.5'],
          [<C>X-Hekato-Route-Reason</C>, 'Why: bandit tier/score, "pinned" (conversation affinity), or a fallback note'],
        ]}
      />
      <Code
        lang="bash"
        code={`curl -i ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"auto","messages":[{"role":"user","content":"Rename this variable: foo -> userCount"}]}' \\
  | grep -i x-hekato`}
      />
      <Callout kind="warning">
        Auto routing must be enabled by the admin (Settings → Auto Routing). When it is off, <C>auto</C> is passed through to the upstream
        unchanged and will usually fail with "model not found".
      </Callout>

      <H2 id="providers">Provider notes</H2>
      <P>
        The pool can mix Kiro, CodeBuddy (Global / China) and Grok (xAI) accounts. All of them serve the same endpoints and the same model
        names; the gateway translates each request into the provider's native format. Which provider actually served you is an admin-side
        detail and does not change the request or response shape.
      </P>
      <Ul>
        <li>
          <strong>Kiro</strong> — Claude models via AWS Builder ID / IAM Identity Center accounts.
        </li>
        <li>
          <strong>CodeBuddy</strong> — Claude models via Tencent CodeBuddy API keys; model IDs advertised by the account appear in{' '}
          <C>/v1/models</C>.
        </li>
        <li>
          <strong>Grok</strong> — xAI models; use the <C>grok-*</C> IDs listed by <C>/v1/models</C>, or let <C>auto</C> pick.
        </li>
      </Ul>
    </>
  )
}

export function Limits() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Every key carries its own quotas. Here is what they mean, what a 429 looks like, and how to read your usage.">Limits & usage</H1>

      <H2 id="per-key">Per-key limits</H2>
      <T
        head={['Limit', 'Scope', 'When exceeded']}
        rows={[
          ['Token quota', 'Cumulative input + output tokens since last reset', '429 token limit exceeded'],
          ['Credit quota', 'Cumulative upstream credits consumed', '429 credit limit exceeded'],
          ['Requests per minute (RPM)', 'Sliding 60-second window', '429 requests per minute limit exceeded'],
          ['Max concurrent', 'In-flight requests at the same time', '429 concurrency limit exceeded'],
        ]}
      />
      <P>
        A value of <C>0</C> means unlimited. Limits are set by the admin per key; usage counters can be reset from the admin panel.
      </P>

      <H2 id="429">429 responses</H2>
      <P>All four limits produce the same shape, in the dialect of the endpoint you called, with a <C>Retry-After</C> header.</P>
      <Code
        lang="http"
        code={`HTTP/1.1 429 Too Many Requests
Retry-After: 1
Content-Type: application/json; charset=utf-8

{"error":{"type":"rate_limit_error","message":"requests per minute limit exceeded"}}`}
      />
      <Ul>
        <li>
          <strong>RPM / concurrency</strong>: wait <C>Retry-After</C> seconds and retry. Most SDKs do this automatically for 429.
        </li>
        <li>
          <strong>Token / credit quota</strong>: retrying will not help until the admin raises the limit or resets usage.
        </li>
      </Ul>

      <H2 id="read-usage">Reading your usage</H2>
      <H3 id="usage-curl">Via the API</H3>
      <Code lang="bash" code={`curl ${base}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`} />
      <Code
        lang="json"
        code={`{
  "name": "my-laptop",
  "keyMasked": "sk-ab…89",
  "enabled": true,
  "requestsCount": 1284,
  "tokensUsed": 5123456,
  "tokenLimit": 20000000,
  "tokenPercent": 0.256,
  "creditsUsed": 12.4,
  "creditLimit": 0,
  "creditPercent": 0,
  "rpmLimit": 60,
  "concurrencyLimit": 4,
  "createdAt": 1758400000,
  "lastUsedAt": 1758499999
}`}
      />
      <H3 id="usage-page">Via the web page</H3>
      <P>
        Open <A href="/usage">{base}/usage</A>, paste the key, and the same numbers are shown as progress bars. No admin login is needed; the key
        is the credential and is only sent to this gateway.
      </P>

      <H2 id="affinity">Conversation affinity</H2>
      <P>
        Multi-turn conversations are pinned to the upstream account that served the first turn. The pin key is{' '}
        <strong>model + system prompt + first user message</strong>, with a 5-minute sliding TTL. This keeps the upstream prompt cache warm, so
        follow-up turns are faster and cheaper.
      </P>
      <Callout title="To benefit from caching">
        Keep the system prompt and the first user message byte-identical across turns. Tools that rewrite the system prompt every request (for
        example by embedding a timestamp) defeat the pin and spread turns across accounts.
      </Callout>
      <P>
        If the pinned account cools down or is excluded, routing falls back to weighted round-robin and re-pins. With <C>auto</C>, a pinned
        conversation also keeps its resolved model (<C>X-Hekato-Route-Reason: pinned</C>).
      </P>
    </>
  )
}

export function Troubleshooting() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="The errors you are most likely to meet and what to do about each.">Troubleshooting</H1>

      <H2 id="401">401 Unauthorized</H2>
      <P>
        Message is <C>Invalid or missing API key</C> or <C>API key disabled</C>.
      </P>
      <Ul>
        <li>
          Check the header: <C>Authorization: Bearer YOUR_API_KEY</C> or <C>x-api-key: YOUR_API_KEY</C>. Some tools want the key in an env var
          named differently (Claude Code: <C>ANTHROPIC_AUTH_TOKEN</C>; Codex: the <C>env_key</C> you configured).
        </li>
        <li>Ask the admin whether the key is enabled and whether "Require API key" is turned on with at least one key.</li>
        <li>Trailing whitespace or a newline copied along with the key is a classic cause.</li>
      </Ul>
      <Code lang="bash" code={`curl -i ${base}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`} />

      <H2 id="429">429 Too Many Requests</H2>
      <P>Read the message:</P>
      <T
        head={['Message', 'Fix']}
        rows={[
          [<C>requests per minute limit exceeded</C>, 'Slow down or ask for a higher RPM. Honour Retry-After.'],
          [<C>concurrency limit exceeded</C>, 'Reduce parallel requests (agents often fan out tool calls).'],
          [<C>token limit exceeded</C>, 'Quota exhausted; admin must raise or reset it.'],
          [<C>credit limit exceeded</C>, 'Same as above, for the credit quota.'],
        ]}
      />
      <P>
        See <A href="/docs/limits">Limits</A> for details.
      </P>

      <H2 id="503">503 No available accounts</H2>
      <P>
        Every account in the pool that can serve the requested model is disabled, cooling down after upstream errors, or out of quota. This is
        an operator-side condition: retry after a minute, and if it persists, tell the admin. Trying a different model (for example{' '}
        <C>claude-haiku-4.5</C> or <C>auto</C>) sometimes helps because tiers map to different accounts.
      </P>

      <H2 id="model-not-found">Model not found</H2>
      <Ul>
        <li>
          List what is actually available: <C>curl {base}/v1/models</C>.
        </li>
        <li>
          Use Anthropic-style names (<C>claude-sonnet-4.5</C>); <C>gpt-4o</C> / <C>gpt-4</C> are the only OpenAI aliases.
        </li>
        <li>
          <C>auto</C> only works when the admin has enabled Auto Routing.
        </li>
        <li>
          A thinking suffix other than the configured one (default <C>-thinking</C>) is treated as part of the model name and fails.
        </li>
      </Ul>

      <H2 id="streaming">Streaming stalls or arrives all at once</H2>
      <P>
        The gateway flushes every SSE event. If you see nothing until the end, a proxy in between is buffering. Disable buffering for the API
        paths:
      </P>
      <Code
        lang="nginx"
        code={`location /v1/ {
    proxy_pass http://hekato:8080;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 600s;
    proxy_set_header Connection "";
    chunked_transfer_encoding on;
}`}
      />
      <Ul>
        <li>
          Caddy: <C>reverse_proxy hekato:8080 {'{ flush_interval -1 }'}</C>
        </li>
        <li>Cloudflare: streaming works on proxied hosts, but keep the request under the 100 s idle timeout on free plans or use long-lived thinking with care.</li>
        <li>
          With curl, add <C>-N</C> to disable its own output buffering.
        </li>
      </Ul>

      <H2 id="cors">CORS</H2>
      <P>
        The gateway answers <C>OPTIONS</C> preflights with <C>Access-Control-Allow-Origin: *</C> and allows the <C>Authorization</C>,
        <C>x-api-key</C>, <C>anthropic-version</C> and <C>anthropic-beta</C> headers, so browser apps can call it directly. If you still get CORS
        errors, a proxy in front is stripping the headers or answering the preflight itself.
      </P>
      <Callout kind="warning">
        Calling the gateway from browser code exposes your API key to anyone who opens DevTools. Use a dedicated key with a small quota.
      </Callout>

      <H2 id="thinking-output">Thinking text shows up in my answers</H2>
      <P>
        You are using a <C>-thinking</C> model with an OpenAI-style client, and the gateway is configured to surface thinking inline. Either drop
        the suffix or ask the admin to switch the thinking output format under Settings → Thinking Mode.
      </P>
    </>
  )
}
