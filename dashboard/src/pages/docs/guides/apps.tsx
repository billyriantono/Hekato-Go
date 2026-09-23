// Chat UIs (Open WebUI, LibreChat) and the full curl reference.
import { A, C, Callout, Code, H1, H2, Ol, P, useDocs } from '../content'

export function OpenWebUiLibreChat() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Both chat front-ends treat the gateway as an OpenAI-compatible provider.">Open WebUI / LibreChat</H1>

      <H2 id="open-webui">Open WebUI</H2>
      <Ol>
        <li>
          Log in as admin and open <strong>Admin Panel → Settings → Connections</strong>.
        </li>
        <li>
          Under <strong>OpenAI API</strong>, click <strong>+</strong> to add a connection.
        </li>
        <li>
          <strong>URL</strong>: <C>{base}/v1</C>
        </li>
        <li>
          <strong>Key</strong>: <C>YOUR_API_KEY</C>
        </li>
        <li>
          Save. The model list is fetched from <C>/v1/models</C>; the Claude models, <C>-thinking</C> variants and <C>auto</C> appear in the
          model picker.
        </li>
      </Ol>
      <P>Or via environment variables when you run the container:</P>
      <Code
        lang="bash"
        code={`docker run -d -p 3000:8080 \\
  -e OPENAI_API_BASE_URL=${base}/v1 \\
  -e OPENAI_API_KEY=YOUR_API_KEY \\
  -v open-webui:/app/backend/data \\
  --name open-webui ghcr.io/open-webui/open-webui:main`}
      />

      <H2 id="librechat">LibreChat</H2>
      <P>
        Add a custom endpoint in <C>librechat.yaml</C>:
      </P>
      <Code
        lang="yaml"
        code={`version: 1.2.1
endpoints:
  custom:
    - name: "Hekato"
      apiKey: "\${HEKATO_API_KEY}"
      baseURL: "${base}/v1"
      models:
        default: ["claude-sonnet-4.5", "claude-opus-4.5", "claude-haiku-4.5", "claude-sonnet-4.5-thinking", "auto"]
        fetch: true
      titleConvo: true
      titleModel: "claude-haiku-4.5"
      modelDisplayLabel: "Hekato"
      dropParams: ["stop", "user", "frequency_penalty", "presence_penalty"]`}
      />
      <Code lang="bash" code={`# .env\nHEKATO_API_KEY=YOUR_API_KEY`} />
      <Callout>
        LibreChat can also use its native Anthropic endpoint: set <C>ANTHROPIC_API_KEY=YOUR_API_KEY</C> and{' '}
        <C>ANTHROPIC_REVERSE_PROXY={base}/v1/messages</C> in <C>.env</C>. This path keeps thinking blocks and prompt caching.
      </Callout>

      <H2 id="verify">Verify</H2>
      <P>
        Pick <C>claude-sonnet-4.5</C> in the model selector, send a message, then confirm the request count on <A href="/usage">/usage</A>.
      </P>
    </>
  )
}

export function Curl() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Copy-paste reference for every endpoint using nothing but curl.">curl reference</H1>

      <H2 id="non-stream">Non-streaming</H2>
      <Code
        lang="bash"
        code={`# Anthropic
curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":256,"messages":[{"role":"user","content":"Hello"}]}'

# OpenAI Chat
curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"Hello"}]}'

# OpenAI Responses
curl ${base}/v1/responses \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","input":"Hello"}'`}
      />

      <H2 id="stream">Streaming</H2>
      <Code
        lang="bash"
        code={`# Anthropic SSE
curl -N ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"Count to five."}]}'

# OpenAI SSE
curl -N ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","stream":true,"messages":[{"role":"user","content":"Count to five."}]}'`}
      />

      <H2 id="tools">Tool use</H2>
      <Code
        lang="bash"
        code={`# Anthropic tools
curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "max_tokens": 512,
    "tools": [{
      "name": "get_weather",
      "description": "Get the current weather for a city",
      "input_schema": {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
    }],
    "messages": [{"role":"user","content":"What is the weather in Jakarta?"}]
  }'

# OpenAI tools
curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the current weather for a city",
        "parameters": {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
      }
    }],
    "messages": [{"role":"user","content":"What is the weather in Jakarta?"}]
  }'`}
      />

      <H2 id="images">Images</H2>
      <Code
        lang="bash"
        code={`IMG=$(base64 -i photo.jpg | tr -d '\\n')

# Anthropic
curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d "{
    \\"model\\": \\"claude-sonnet-4.5\\",
    \\"max_tokens\\": 256,
    \\"messages\\": [{\\"role\\":\\"user\\",\\"content\\":[
      {\\"type\\":\\"image\\",\\"source\\":{\\"type\\":\\"base64\\",\\"media_type\\":\\"image/jpeg\\",\\"data\\":\\"$IMG\\"}},
      {\\"type\\":\\"text\\",\\"text\\":\\"Describe this image.\\"}
    ]}]
  }"

# OpenAI
curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "{
    \\"model\\": \\"claude-sonnet-4.5\\",
    \\"messages\\": [{\\"role\\":\\"user\\",\\"content\\":[
      {\\"type\\":\\"image_url\\",\\"image_url\\":{\\"url\\":\\"data:image/jpeg;base64,$IMG\\"}},
      {\\"type\\":\\"text\\",\\"text\\":\\"Describe this image.\\"}
    ]}]
  }"`}
      />

      <H2 id="thinking">Thinking</H2>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":2048},"messages":[{"role":"user","content":"Prove that sqrt(2) is irrational."}]}'`}
      />

      <H2 id="count-tokens">Count tokens</H2>
      <Code
        lang="bash"
        code={`curl ${base}/v1/messages/count_tokens \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"How many tokens is this?"}]}'`}
      />

      <H2 id="models">Models</H2>
      <Code lang="bash" code={`curl ${base}/v1/models | jq -r '.data[].id'`} />

      <H2 id="usage">Usage</H2>
      <Code lang="bash" code={`curl ${base}/v1/usage -H "Authorization: Bearer YOUR_API_KEY" | jq`} />

      <H2 id="health">Health</H2>
      <Code lang="bash" code={`curl ${base}/health`} />

      <H2 id="auto">Auto routing headers</H2>
      <Code
        lang="bash"
        code={`curl -si ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello"}]}' | grep -i '^x-hekato'`}
      />
    </>
  )
}
