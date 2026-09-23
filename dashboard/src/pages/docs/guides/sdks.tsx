// SDK guides: OpenAI SDK, Anthropic SDK, LangChain.
import { C, Callout, Code, H1, H2, H3, P, useDocs } from '../content'

export function OpenAISdk() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="The official OpenAI SDKs only need a base_url and an api_key.">OpenAI SDK</H1>

      <H2 id="python">Python</H2>
      <Code lang="bash" code={`pip install openai`} />
      <Code
        lang="python"
        code={`from openai import OpenAI

client = OpenAI(base_url="${base}/v1", api_key="YOUR_API_KEY")

resp = client.chat.completions.create(
    model="claude-sonnet-4.5",
    messages=[{"role": "user", "content": "Say hello in one sentence."}],
)
print(resp.choices[0].message.content)`}
      />
      <H3 id="python-stream">Streaming</H3>
      <Code
        lang="python"
        code={`stream = client.chat.completions.create(
    model="claude-sonnet-4.5",
    messages=[{"role": "user", "content": "Count to five."}],
    stream=True,
)
for chunk in stream:
    delta = chunk.choices[0].delta.content
    if delta:
        print(delta, end="", flush=True)`}
      />
      <H3 id="python-responses">Responses API</H3>
      <Code
        lang="python"
        code={`resp = client.responses.create(
    model="claude-sonnet-4.5",
    instructions="Be brief.",
    input="What is an AI gateway?",
)
print(resp.output_text)`}
      />

      <H2 id="node">Node / TypeScript</H2>
      <Code lang="bash" code={`npm install openai`} />
      <Code
        lang="typescript"
        code={`import OpenAI from 'openai'

const client = new OpenAI({ baseURL: '${base}/v1', apiKey: 'YOUR_API_KEY' })

const resp = await client.chat.completions.create({
  model: 'claude-sonnet-4.5',
  messages: [{ role: 'user', content: 'Say hello in one sentence.' }],
})
console.log(resp.choices[0].message.content)`}
      />
      <H3 id="node-stream">Streaming</H3>
      <Code
        lang="typescript"
        code={`const stream = await client.chat.completions.create({
  model: 'claude-sonnet-4.5',
  messages: [{ role: 'user', content: 'Count to five.' }],
  stream: true,
})
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '')
}`}
      />
      <H3 id="node-responses">Responses API</H3>
      <Code
        lang="typescript"
        code={`const r = await client.responses.create({
  model: 'claude-sonnet-4.5',
  instructions: 'Be brief.',
  input: 'What is an AI gateway?',
})
console.log(r.output_text)`}
      />

      <H2 id="env">Environment variables</H2>
      <P>Both SDKs also read these, so you can leave the constructor empty:</P>
      <Code
        lang="bash"
        code={`export OPENAI_BASE_URL=${base}/v1
export OPENAI_API_KEY=YOUR_API_KEY`}
      />
      <Callout>
        Thinking: use <C>model="claude-sonnet-4.5-thinking"</C>. The OpenAI wire format has no thinking parameter, so the suffix is the switch.
      </Callout>
    </>
  )
}

export function AnthropicSdk() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="The official Anthropic SDKs work unchanged; pass the gateway as base_url (no /v1).">Anthropic SDK</H1>

      <H2 id="python">Python</H2>
      <Code lang="bash" code={`pip install anthropic`} />
      <Code
        lang="python"
        code={`import anthropic

client = anthropic.Anthropic(base_url="${base}", api_key="YOUR_API_KEY")

msg = client.messages.create(
    model="claude-sonnet-4.5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Say hello in one sentence."}],
)
print(msg.content[0].text)`}
      />
      <H3 id="python-stream">Streaming</H3>
      <Code
        lang="python"
        code={`with client.messages.stream(
    model="claude-sonnet-4.5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Count to five."}],
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)`}
      />
      <H3 id="python-thinking">Thinking block</H3>
      <Code
        lang="python"
        code={`msg = client.messages.create(
    model="claude-sonnet-4.5",
    max_tokens=4096,
    thinking={"type": "enabled", "budget_tokens": 2048},
    messages=[{"role": "user", "content": "Prove that sqrt(2) is irrational."}],
)
for block in msg.content:
    if block.type == "thinking":
        print("[thinking]", block.thinking[:200], "...")
    elif block.type == "text":
        print(block.text)`}
      />

      <H2 id="node">Node / TypeScript</H2>
      <Code lang="bash" code={`npm install @anthropic-ai/sdk`} />
      <Code
        lang="typescript"
        code={`import Anthropic from '@anthropic-ai/sdk'

const client = new Anthropic({ baseURL: '${base}', apiKey: 'YOUR_API_KEY' })

const msg = await client.messages.create({
  model: 'claude-sonnet-4.5',
  max_tokens: 256,
  messages: [{ role: 'user', content: 'Say hello in one sentence.' }],
})
console.log(msg.content[0].type === 'text' ? msg.content[0].text : msg.content)`}
      />
      <H3 id="node-stream">Streaming</H3>
      <Code
        lang="typescript"
        code={`const stream = client.messages.stream({
  model: 'claude-sonnet-4.5',
  max_tokens: 256,
  messages: [{ role: 'user', content: 'Count to five.' }],
})
stream.on('text', (t) => process.stdout.write(t))
await stream.finalMessage()`}
      />
      <H3 id="node-thinking">Thinking block</H3>
      <Code
        lang="typescript"
        code={`const msg = await client.messages.create({
  model: 'claude-sonnet-4.5',
  max_tokens: 4096,
  thinking: { type: 'enabled', budget_tokens: 2048 },
  messages: [{ role: 'user', content: 'Prove that sqrt(2) is irrational.' }],
})`}
      />

      <H2 id="env">Environment variables</H2>
      <Code
        lang="bash"
        code={`export ANTHROPIC_BASE_URL=${base}
export ANTHROPIC_API_KEY=YOUR_API_KEY`}
      />
      <Callout>
        Token counting: <C>client.messages.count_tokens(...)</C> hits <C>/v1/messages/count_tokens</C> and returns a local estimate.
      </Callout>
    </>
  )
}

export function LangChain() {
  const { base } = useDocs()
  return (
    <>
      <H1 lead="Use either the OpenAI or the Anthropic chat model class; both accept a custom base URL.">LangChain</H1>

      <H2 id="openai">ChatOpenAI</H2>
      <Code lang="bash" code={`pip install langchain-openai`} />
      <Code
        lang="python"
        code={`from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="${base}/v1",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    temperature=0,
)
print(llm.invoke("Say hello in one sentence.").content)

# streaming
for chunk in llm.stream("Count to five."):
    print(chunk.content, end="", flush=True)`}
      />

      <H2 id="anthropic">ChatAnthropic</H2>
      <Code lang="bash" code={`pip install langchain-anthropic`} />
      <Code
        lang="python"
        code={`from langchain_anthropic import ChatAnthropic

llm = ChatAnthropic(
    base_url="${base}",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    max_tokens=1024,
)
print(llm.invoke("Say hello in one sentence.").content)

# extended thinking
thinking_llm = ChatAnthropic(
    base_url="${base}",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    max_tokens=4096,
    thinking={"type": "enabled", "budget_tokens": 2048},
)`}
      />

      <H2 id="tools">Tool calling</H2>
      <P>Tool binding works the same on both classes; the gateway translates tool schemas and tool results for the upstream.</P>
      <Code
        lang="python"
        code={`from langchain_core.tools import tool

@tool
def add(a: int, b: int) -> int:
    """Add two integers."""
    return a + b

agent = llm.bind_tools([add])
print(agent.invoke("What is 2 + 3? Use the tool.").tool_calls)`}
      />
      <Callout>
        JavaScript: <C>@langchain/openai</C> takes <C>{'configuration: { baseURL }'}</C>; <C>@langchain/anthropic</C> takes{' '}
        <C>{'clientOptions: { baseURL }'}</C>.
      </Callout>
    </>
  )
}
