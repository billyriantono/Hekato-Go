import{$t as e,Cn as t,Ln as n,Mn as r,Pn as i,R as a,jn as o,kn as s,ln as c,nn as l,yn as ee}from"./useTimeout-B78s_OAX.js";import{n as u}from"./common-_LZW0J15.js";import{r as d,w as f}from"./index-Csblg4TZ.js";import{a as p,i as m,n as h,o as g,r as _,t as v}from"./table-C1sWkubL.js";var y=n(i(),1),b=r(),x=`YOUR_API_KEY`,S=(0,y.createContext)({base:``,apiKey:``}),C=()=>(0,y.useContext)(S);function w({lang:e,code:t,className:n}){let{apiKey:r}=C(),i=r?t.replaceAll(x,r):t;return(0,b.jsxs)(`div`,{className:o(`my-4 overflow-hidden rounded-lg border bg-muted/50`,n),children:[(0,b.jsxs)(`div`,{className:`flex items-center justify-between border-b bg-muted/60 py-1 pr-1 pl-3`,children:[(0,b.jsx)(`span`,{className:`font-mono text-[11px] uppercase tracking-wide text-muted-foreground`,children:e}),(0,b.jsx)(u,{value:i})]}),(0,b.jsx)(`pre`,{className:`overflow-x-auto p-3 font-mono text-[13px] leading-relaxed`,children:(0,b.jsx)(`code`,{children:i})})]})}function T({kind:n=`info`,title:r,children:i}){let a=n===`warning`;return(0,b.jsxs)(`div`,{className:o(`my-4 flex gap-3 rounded-lg border p-3 text-sm`,a?`border-amber-500/40 bg-amber-500/10`:`border-sky-500/40 bg-sky-500/10`),children:[a?(0,b.jsx)(t,{className:`mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400`}):(0,b.jsx)(e,{className:`mt-0.5 size-4 shrink-0 text-sky-600 dark:text-sky-400`}),(0,b.jsxs)(`div`,{className:`min-w-0 space-y-1 [&_code]:rounded [&_code]:bg-background/60 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px]`,children:[r&&(0,b.jsx)(`div`,{className:`font-medium`,children:r}),(0,b.jsx)(`div`,{children:i})]})]})}var E=({children:e,lead:t})=>(0,b.jsxs)(`header`,{className:`mb-8`,children:[(0,b.jsx)(`h1`,{className:`text-3xl font-semibold tracking-tight`,children:e}),t&&(0,b.jsx)(`p`,{className:`mt-2 text-muted-foreground`,children:t})]}),D=({id:e,children:t})=>(0,b.jsx)(`h2`,{id:e,className:`mt-10 mb-3 scroll-mt-20 border-b pb-1.5 text-xl font-semibold tracking-tight`,children:t}),O=({id:e,children:t})=>(0,b.jsx)(`h3`,{id:e,className:`mt-6 mb-2 scroll-mt-20 text-base font-semibold`,children:t}),k=({children:e})=>(0,b.jsx)(`p`,{className:`my-3 leading-7`,children:e}),A=({children:e})=>(0,b.jsx)(`ul`,{className:`my-3 list-disc space-y-1.5 pl-6 leading-7`,children:e}),j=({children:e})=>(0,b.jsx)(`ol`,{className:`my-3 list-decimal space-y-1.5 pl-6 leading-7`,children:e}),M=({children:e})=>(0,b.jsx)(`code`,{className:`rounded bg-muted px-1 py-0.5 font-mono text-[12.5px]`,children:e}),N=({href:e,children:t})=>(0,b.jsx)(`a`,{href:e,className:`font-medium text-primary underline underline-offset-4`,children:t});function P({head:e,rows:t}){return(0,b.jsx)(`div`,{className:`my-4 rounded-lg border`,children:(0,b.jsxs)(v,{children:[(0,b.jsx)(p,{children:(0,b.jsx)(g,{children:e.map((e,t)=>(0,b.jsx)(m,{children:e},t))})}),(0,b.jsx)(h,{children:t.map((e,t)=>(0,b.jsx)(g,{children:e.map((e,t)=>(0,b.jsx)(_,{className:`whitespace-normal align-top`,children:e},t))},t))})]})})}function F(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Hekato-Go is an AI gateway. It exposes Anthropic- and OpenAI-compatible APIs in front of pools of Kiro, CodeBuddy and Grok accounts, with load balancing, per-key limits and smart routing.`,children:`Getting started`}),(0,b.jsx)(D,{id:`what`,children:`What this gateway does`}),(0,b.jsx)(k,{children:`You talk to one host using the API shape you already know (Anthropic Messages, OpenAI Chat Completions or OpenAI Responses). The gateway picks an upstream account, translates the request, streams the answer back and tracks your usage. Any tool that can point at a custom Anthropic or OpenAI base URL works unchanged.`}),(0,b.jsx)(D,{id:`base-url`,children:`Base URL`}),(0,b.jsx)(w,{lang:`text`,code:e}),(0,b.jsxs)(k,{children:[`Anthropic-style clients use the base URL as-is (they append `,(0,b.jsx)(M,{children:`/v1/messages`}),`). OpenAI-style clients usually want `,(0,b.jsxs)(M,{children:[e,`/v1`]}),`.`]}),(0,b.jsx)(D,{id:`api-key`,children:`Get an API key`}),(0,b.jsxs)(k,{children:[`Keys are created by the gateway administrator under `,(0,b.jsx)(N,{href:`/admin`,children:`Admin`}),` → API Keys. Each key can carry its own token / credit quota, requests-per-minute and concurrency limits. Send it as `,(0,b.jsx)(M,{children:`Authorization: Bearer YOUR_API_KEY`}),` or `,(0,b.jsx)(M,{children:`X-Api-Key: YOUR_API_KEY`}),`.`]}),(0,b.jsx)(T,{children:`Paste your key into the field at the top of this page and every snippet below is rewritten with it, ready to copy.`}),(0,b.jsx)(D,{id:`anthropic-30s`,children:`30 seconds: Anthropic Messages`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "Say hello in one sentence."}]
  }'`}),(0,b.jsx)(D,{id:`openai-30s`,children:`30 seconds: OpenAI Chat Completions`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [{"role": "user", "content": "Say hello in one sentence."}]
  }'`}),(0,b.jsx)(D,{id:`check-usage`,children:`Check your usage`}),(0,b.jsxs)(k,{children:[`Open `,(0,b.jsxs)(N,{href:`/usage`,children:[e,`/usage`]}),`, paste your key, and see quota, limits and current consumption. The same data is available as JSON from `,(0,b.jsx)(M,{children:`GET /v1/usage`}),` (see `,(0,b.jsx)(N,{href:`/docs/limits`,children:`Limits`}),`).`]}),(0,b.jsx)(D,{id:`next`,children:`Next steps`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(N,{href:`/docs/endpoints`,children:`Endpoints`}),` — every route, streaming, error format.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(N,{href:`/docs/models`,children:`Models`}),` — model names, thinking mode, the `,(0,b.jsx)(M,{children:`auto`}),` model.`]}),(0,b.jsx)(`li`,{children:`Tool guides in the sidebar — Claude Code, Codex, Cursor, SDKs and more.`})]})]})}function I(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Every public route served by the gateway, how to authenticate, and how errors look in each dialect.`,children:`Endpoints`}),(0,b.jsx)(D,{id:`overview`,children:`Overview`}),(0,b.jsx)(P,{head:[`Method`,`Path`,`Purpose`,`Auth`],rows:[[`POST`,(0,b.jsx)(M,{children:`/v1/messages`}),`Anthropic Messages API (streaming and non-streaming)`,`API key`],[`POST`,(0,b.jsx)(M,{children:`/v1/messages/count_tokens`}),`Estimate input tokens for an Anthropic request`,`API key`],[`POST`,(0,b.jsx)(M,{children:`/v1/chat/completions`}),`OpenAI Chat Completions API`,`API key`],[`POST`,(0,b.jsx)(M,{children:`/v1/responses`}),`OpenAI Responses API`,`API key`],[`GET`,(0,b.jsx)(M,{children:`/v1/models`}),`List available model IDs`,`none`],[`GET`,(0,b.jsx)(M,{children:`/v1/usage`}),`Quota and usage of the calling key (JSON)`,`API key`],[`GET`,(0,b.jsx)(M,{children:`/health`}),`Liveness: status, version, uptime`,`none`]]}),(0,b.jsxs)(k,{children:[`The `,(0,b.jsx)(M,{children:`/v1`}),` prefix is optional: `,(0,b.jsx)(M,{children:`/messages`}),`, `,(0,b.jsx)(M,{children:`/chat/completions`}),` and `,(0,b.jsx)(M,{children:`/responses`}),` resolve to the same handlers.`,(0,b.jsx)(M,{children:`/anthropic/v1/messages`}),` is accepted too for clients that hard-code that layout.`]}),(0,b.jsx)(D,{id:`auth`,children:`Authentication`}),(0,b.jsx)(k,{children:`Send the key in either header; both dialects accept both forms.`}),(0,b.jsx)(w,{lang:`http`,code:`Authorization: Bearer YOUR_API_KEY
# or
X-Api-Key: YOUR_API_KEY`}),(0,b.jsxs)(T,{kind:`warning`,children:[`Keys are checked on every request. A missing, unknown or disabled key returns `,(0,b.jsx)(M,{children:`401`}),`; a key over its quota or rate limit returns`,` `,(0,b.jsx)(M,{children:`429`}),` with a `,(0,b.jsx)(M,{children:`Retry-After`}),` header.`]}),(0,b.jsx)(D,{id:`messages`,children:`POST /v1/messages`}),(0,b.jsxs)(k,{children:[`Standard Anthropic Messages request body: `,(0,b.jsx)(M,{children:`model`}),`, `,(0,b.jsx)(M,{children:`max_tokens`}),`, `,(0,b.jsx)(M,{children:`messages`}),`, optional `,(0,b.jsx)(M,{children:`system`}),`, `,(0,b.jsx)(M,{children:`tools`}),`,`,(0,b.jsx)(M,{children:`thinking`}),`, `,(0,b.jsx)(M,{children:`stream`}),`. Responses use the Anthropic shape (`,(0,b.jsx)(M,{children:`content`}),` blocks, `,(0,b.jsx)(M,{children:`stop_reason`}),`, `,(0,b.jsx)(M,{children:`usage`}),`).`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":512,"system":"Be brief.","messages":[{"role":"user","content":"What is an AI gateway?"}]}'`}),(0,b.jsx)(D,{id:`count-tokens`,children:`POST /v1/messages/count_tokens`}),(0,b.jsxs)(k,{children:[`Same body as `,(0,b.jsx)(M,{children:`/v1/messages`}),` minus `,(0,b.jsx)(M,{children:`max_tokens`}),`. Returns `,(0,b.jsx)(M,{children:`{"input_tokens": N}`}),`. The count is a local estimate, useful for budgeting before you send the real request.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages/count_tokens \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"How many tokens is this?"}]}'`}),(0,b.jsx)(D,{id:`chat-completions`,children:`POST /v1/chat/completions`}),(0,b.jsxs)(k,{children:[`OpenAI Chat Completions request: `,(0,b.jsx)(M,{children:`model`}),`, `,(0,b.jsx)(M,{children:`messages`}),`, optional `,(0,b.jsx)(M,{children:`tools`}),`, `,(0,b.jsx)(M,{children:`stream`}),`, `,(0,b.jsx)(M,{children:`max_tokens`}),` /`,(0,b.jsx)(M,{children:`max_completion_tokens`}),`, `,(0,b.jsx)(M,{children:`temperature`}),`. Image parts (`,(0,b.jsx)(M,{children:`image_url`}),`) and tool calls are translated to the upstream format.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"system","content":"Be brief."},{"role":"user","content":"What is an AI gateway?"}]}'`}),(0,b.jsx)(D,{id:`responses`,children:`POST /v1/responses`}),(0,b.jsxs)(k,{children:[`OpenAI Responses API, used by Codex CLI and newer SDK code paths. Accepts `,(0,b.jsx)(M,{children:`input`}),` as a string or an item list, `,(0,b.jsx)(M,{children:`instructions`}),`,`,(0,b.jsx)(M,{children:`tools`}),`, `,(0,b.jsx)(M,{children:`previous_response_id`}),` and `,(0,b.jsx)(M,{children:`stream`}),`. Conversation state referenced by `,(0,b.jsx)(M,{children:`previous_response_id`}),` is kept in memory on the gateway.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/responses \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","input":"What is an AI gateway?"}'`}),(0,b.jsx)(D,{id:`models-endpoint`,children:`GET /v1/models`}),(0,b.jsxs)(k,{children:[`Lists the models the account pool currently advertises, plus the aliases `,(0,b.jsx)(M,{children:`auto`}),`, `,(0,b.jsx)(M,{children:`gpt-4o`}),` and `,(0,b.jsx)(M,{children:`gpt-4`}),`, and a`,` `,(0,b.jsx)(M,{children:`-thinking`}),` variant for each Claude model. No key is needed.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/models`}),(0,b.jsx)(D,{id:`usage-endpoint`,children:`GET /v1/usage`}),(0,b.jsx)(k,{children:`Returns the calling key's name, masked value, limits and consumption. Powers the public /usage page.`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`}),(0,b.jsx)(D,{id:`health`,children:`GET /health`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/health\n# {"status":"ok","version":"...","uptime":12345}`}),(0,b.jsx)(D,{id:`streaming`,children:`Streaming (SSE)`}),(0,b.jsxs)(k,{children:[`Set `,(0,b.jsx)(M,{children:`"stream": true`}),` in any chat endpoint. The response is `,(0,b.jsx)(M,{children:`text/event-stream`}),` using the native event format of the dialect you called:`]}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[`Anthropic: `,(0,b.jsx)(M,{children:`message_start`}),`, `,(0,b.jsx)(M,{children:`content_block_start`}),`, `,(0,b.jsx)(M,{children:`content_block_delta`}),`, `,(0,b.jsx)(M,{children:`message_delta`}),`, `,(0,b.jsx)(M,{children:`message_stop`}),`.`]}),(0,b.jsxs)(`li`,{children:[`OpenAI Chat: `,(0,b.jsxs)(M,{children:[`data: `,`{...chat.completion.chunk...}`]}),` lines terminated by `,(0,b.jsx)(M,{children:`data: [DONE]`}),`.`]}),(0,b.jsxs)(`li`,{children:[`OpenAI Responses: `,(0,b.jsx)(M,{children:`response.created`}),`, `,(0,b.jsx)(M,{children:`response.output_text.delta`}),`, `,(0,b.jsx)(M,{children:`response.completed`}),`.`]})]}),(0,b.jsx)(w,{lang:`bash`,code:`curl -N ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","stream":true,"messages":[{"role":"user","content":"Count to five."}]}'`}),(0,b.jsxs)(T,{children:[`If you sit behind nginx, Cloudflare or another reverse proxy, disable response buffering for these paths, otherwise tokens arrive in one burst at the end. See `,(0,b.jsx)(N,{href:`/docs/troubleshooting#streaming`,children:`Troubleshooting`}),`.`]}),(0,b.jsx)(D,{id:`errors`,children:`Error format`}),(0,b.jsx)(k,{children:`Errors follow the dialect of the endpoint you called.`}),(0,b.jsx)(O,{id:`errors-anthropic`,children:`Anthropic endpoints`}),(0,b.jsx)(w,{lang:`json`,code:`{"type":"error","error":{"type":"authentication_error","message":"Invalid or missing API key"}}`}),(0,b.jsx)(O,{id:`errors-openai`,children:`OpenAI endpoints`}),(0,b.jsx)(w,{lang:`json`,code:`{"error":{"type":"authentication_error","message":"Invalid or missing API key"}}`}),(0,b.jsx)(D,{id:`status-codes`,children:`Status codes`}),(0,b.jsx)(P,{head:[`Status`,`Type`,`Meaning`],rows:[[`400`,(0,b.jsx)(M,{children:`invalid_request_error`}),`Body is not valid JSON or fails validation`],[`401`,(0,b.jsx)(M,{children:`authentication_error`}),`Key missing, unknown or disabled`],[`429`,(0,b.jsx)(M,{children:`rate_limit_error`}),`Token / credit quota, RPM or concurrency limit hit. Retry-After header is set.`],[`503`,(0,b.jsx)(M,{children:`api_error`}),`No available accounts in the pool for this model`]]}),(0,b.jsxs)(k,{children:[`On `,(0,b.jsx)(M,{children:`429`}),` the gateway sets `,(0,b.jsx)(M,{children:`Retry-After: 1`}),` (seconds). Back off at least that long; for quota exhaustion, ask your admin to raise the limit or reset usage.`]})]})}function L(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`How model names are resolved, how to turn on extended thinking, and how the auto model routes for you.`,children:`Models`}),(0,b.jsx)(D,{id:`names`,children:`Model names`}),(0,b.jsxs)(k,{children:[`Use Anthropic-style IDs. The canonical names are `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`, `,(0,b.jsx)(M,{children:`claude-opus-4.5`}),` and `,(0,b.jsx)(M,{children:`claude-haiku-4.5`}),`; whatever the pool currently advertises is listed by `,(0,b.jsx)(M,{children:`GET /v1/models`}),`.`]}),(0,b.jsx)(P,{head:[`You send`,`Resolves to`,`Note`],rows:[[(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`Canonical form`],[(0,b.jsx)(M,{children:`claude-sonnet-4-5`}),(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`Dash form is normalized`],[(0,b.jsx)(M,{children:`claude-sonnet-4-5-20250929`}),(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`Dated suffixes are stripped`],[(0,b.jsx)(M,{children:`gpt-4o`}),(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`OpenAI alias for tools that hard-code GPT names`],[(0,b.jsx)(M,{children:`gpt-4`}),(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`Same alias`],[(0,b.jsx)(M,{children:`auto`}),`chosen per request`,`See Auto routing below`]]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/models | jq '.data[].id'`}),(0,b.jsx)(D,{id:`thinking`,children:`Thinking mode`}),(0,b.jsx)(k,{children:`Two ways to enable extended thinking, both work on every endpoint:`}),(0,b.jsx)(O,{id:`thinking-suffix`,children:`1. Model suffix`}),(0,b.jsxs)(k,{children:[`Append `,(0,b.jsx)(M,{children:`-thinking`}),` (the default suffix; the admin can change it under Settings → Thinking Mode). This is the only option for OpenAI-style clients.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5-thinking","messages":[{"role":"user","content":"Prove that sqrt(2) is irrational."}]}'`}),(0,b.jsx)(O,{id:`thinking-block`,children:`2. Anthropic thinking block`}),(0,b.jsxs)(k,{children:[`On `,(0,b.jsx)(M,{children:`/v1/messages`}),`, a top-level `,(0,b.jsx)(M,{children:`thinking`}),` object enables it automatically: `,(0,b.jsx)(M,{children:`{"type":"enabled","budget_tokens":2048}`}),` `,`or `,(0,b.jsx)(M,{children:`{"type":"adaptive"}`}),`.`]}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4.5",
    "max_tokens": 4096,
    "thinking": {"type": "enabled", "budget_tokens": 2048},
    "messages": [{"role": "user", "content": "Prove that sqrt(2) is irrational."}]
  }'`}),(0,b.jsxs)(T,{children:[`How thinking is surfaced (Anthropic `,(0,b.jsx)(M,{children:`thinking`}),` content blocks, or inline `,(0,b.jsx)(M,{children:`<thinking>`}),` tags in text) is a gateway-wide setting chosen by the admin.`]}),(0,b.jsx)(D,{id:`auto`,children:`The auto model`}),(0,b.jsxs)(k,{children:[`Send `,(0,b.jsx)(M,{children:`"model": "auto"`}),` and the gateway classifies the request (input size, tool count, turns, images, thinking) into a fast / balanced / strong tier, then a learning bandit picks the best (account, model) pair inside that tier using observed success rate, latency and remaining quota.`]}),(0,b.jsx)(k,{children:`The decision is reported in two response headers:`}),(0,b.jsx)(P,{head:[`Header`,`Value`],rows:[[(0,b.jsx)(M,{children:`X-Hekato-Routed-Model`}),`The concrete model that served the request, e.g. claude-haiku-4.5`],[(0,b.jsx)(M,{children:`X-Hekato-Route-Reason`}),`Why: bandit tier/score, "pinned" (conversation affinity), or a fallback note`]]}),(0,b.jsx)(w,{lang:`bash`,code:`curl -i ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"auto","messages":[{"role":"user","content":"Rename this variable: foo -> userCount"}]}' \\
  | grep -i x-hekato`}),(0,b.jsxs)(T,{kind:`warning`,children:[`Auto routing must be enabled by the admin (Settings → Auto Routing). When it is off, `,(0,b.jsx)(M,{children:`auto`}),` is passed through to the upstream unchanged and will usually fail with "model not found".`]}),(0,b.jsx)(D,{id:`providers`,children:`Provider notes`}),(0,b.jsx)(k,{children:`The pool can mix Kiro, CodeBuddy, Grok, OpenCode Zen, and OpenCode Go accounts. All of them serve the same endpoints and the same model names; the gateway translates each request into the provider's native format. Which provider actually served you is an admin-side detail and does not change the request or response shape.`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Kiro`}),` — Claude models via AWS Builder ID / IAM Identity Center accounts.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`CodeBuddy`}),` — Claude models via Tencent CodeBuddy API keys; model IDs advertised by the account appear in`,` `,(0,b.jsx)(M,{children:`/v1/models`}),`.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Grok`}),` — xAI models; use the `,(0,b.jsx)(M,{children:`grok-*`}),` IDs listed by `,(0,b.jsx)(M,{children:`/v1/models`}),`, or let `,(0,b.jsx)(M,{children:`auto`}),` pick.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`OpenCode Zen`}),` — free-tier models via opencode.ai/zen; the gateway injects the required fingerprint tools automatically.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`OpenCode Go`}),` — subscription-tier ($5+/mo) models via opencode.ai/zen/go; same fingerprint requirement, auto-injected.`]})]})]})}function R(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Every key carries its own quotas. Here is what they mean, what a 429 looks like, and how to read your usage.`,children:`Limits & usage`}),(0,b.jsx)(D,{id:`per-key`,children:`Per-key limits`}),(0,b.jsx)(P,{head:[`Limit`,`Scope`,`When exceeded`],rows:[[`Token quota`,`Cumulative input + output tokens since last reset`,`429 token limit exceeded`],[`Credit quota`,`Cumulative upstream credits consumed`,`429 credit limit exceeded`],[`Requests per minute (RPM)`,`Sliding 60-second window`,`429 requests per minute limit exceeded`],[`Max concurrent`,`In-flight requests at the same time`,`429 concurrency limit exceeded`]]}),(0,b.jsxs)(k,{children:[`A value of `,(0,b.jsx)(M,{children:`0`}),` means unlimited. Limits are set by the admin per key; usage counters can be reset from the admin panel.`]}),(0,b.jsx)(D,{id:`429`,children:`429 responses`}),(0,b.jsxs)(k,{children:[`All four limits produce the same shape, in the dialect of the endpoint you called, with a `,(0,b.jsx)(M,{children:`Retry-After`}),` header.`]}),(0,b.jsx)(w,{lang:`http`,code:`HTTP/1.1 429 Too Many Requests
Retry-After: 1
Content-Type: application/json; charset=utf-8

{"error":{"type":"rate_limit_error","message":"requests per minute limit exceeded"}}`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`RPM / concurrency`}),`: wait `,(0,b.jsx)(M,{children:`Retry-After`}),` seconds and retry. Most SDKs do this automatically for 429.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Token / credit quota`}),`: retrying will not help until the admin raises the limit or resets usage.`]})]}),(0,b.jsx)(D,{id:`read-usage`,children:`Reading your usage`}),(0,b.jsx)(O,{id:`usage-curl`,children:`Via the API`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`}),(0,b.jsx)(w,{lang:`json`,code:`{
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
}`}),(0,b.jsx)(O,{id:`usage-page`,children:`Via the web page`}),(0,b.jsxs)(k,{children:[`Open `,(0,b.jsxs)(N,{href:`/usage`,children:[e,`/usage`]}),`, paste the key, and the same numbers are shown as progress bars. No admin login is needed; the key is the credential and is only sent to this gateway.`]}),(0,b.jsx)(D,{id:`affinity`,children:`Conversation affinity`}),(0,b.jsxs)(k,{children:[`Multi-turn conversations are pinned to the upstream account that served the first turn. The pin key is`,` `,(0,b.jsx)(`strong`,{children:`model + system prompt + first user message`}),`, with a 5-minute sliding TTL. This keeps the upstream prompt cache warm, so follow-up turns are faster and cheaper.`]}),(0,b.jsx)(T,{title:`To benefit from caching`,children:`Keep the system prompt and the first user message byte-identical across turns. Tools that rewrite the system prompt every request (for example by embedding a timestamp) defeat the pin and spread turns across accounts.`}),(0,b.jsxs)(k,{children:[`If the pinned account cools down or is excluded, routing falls back to weighted round-robin and re-pins. With `,(0,b.jsx)(M,{children:`auto`}),`, a pinned conversation also keeps its resolved model (`,(0,b.jsx)(M,{children:`X-Hekato-Route-Reason: pinned`}),`).`]})]})}function z(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`The errors you are most likely to meet and what to do about each.`,children:`Troubleshooting`}),(0,b.jsx)(D,{id:`401`,children:`401 Unauthorized`}),(0,b.jsxs)(k,{children:[`Message is `,(0,b.jsx)(M,{children:`Invalid or missing API key`}),` or `,(0,b.jsx)(M,{children:`API key disabled`}),`.`]}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[`Check the header: `,(0,b.jsx)(M,{children:`Authorization: Bearer YOUR_API_KEY`}),` or `,(0,b.jsx)(M,{children:`x-api-key: YOUR_API_KEY`}),`. Some tools want the key in an env var named differently (Claude Code: `,(0,b.jsx)(M,{children:`ANTHROPIC_AUTH_TOKEN`}),`; Codex: the `,(0,b.jsx)(M,{children:`env_key`}),` you configured).`]}),(0,b.jsx)(`li`,{children:`Ask the admin whether the key is enabled and whether "Require API key" is turned on with at least one key.`}),(0,b.jsx)(`li`,{children:`Trailing whitespace or a newline copied along with the key is a classic cause.`})]}),(0,b.jsx)(w,{lang:`bash`,code:`curl -i ${e}/v1/usage -H "Authorization: Bearer YOUR_API_KEY"`}),(0,b.jsx)(D,{id:`429`,children:`429 Too Many Requests`}),(0,b.jsx)(k,{children:`Read the message:`}),(0,b.jsx)(P,{head:[`Message`,`Fix`],rows:[[(0,b.jsx)(M,{children:`requests per minute limit exceeded`}),`Slow down or ask for a higher RPM. Honour Retry-After.`],[(0,b.jsx)(M,{children:`concurrency limit exceeded`}),`Reduce parallel requests (agents often fan out tool calls).`],[(0,b.jsx)(M,{children:`token limit exceeded`}),`Quota exhausted; admin must raise or reset it.`],[(0,b.jsx)(M,{children:`credit limit exceeded`}),`Same as above, for the credit quota.`]]}),(0,b.jsxs)(k,{children:[`See `,(0,b.jsx)(N,{href:`/docs/limits`,children:`Limits`}),` for details.`]}),(0,b.jsx)(D,{id:`503`,children:`503 No available accounts`}),(0,b.jsxs)(k,{children:[`Every account in the pool that can serve the requested model is disabled, cooling down after upstream errors, or out of quota. This is an operator-side condition: retry after a minute, and if it persists, tell the admin. Trying a different model (for example`,` `,(0,b.jsx)(M,{children:`claude-haiku-4.5`}),` or `,(0,b.jsx)(M,{children:`auto`}),`) sometimes helps because tiers map to different accounts.`]}),(0,b.jsx)(D,{id:`model-not-found`,children:`Model not found`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[`List what is actually available: `,(0,b.jsxs)(M,{children:[`curl `,e,`/v1/models`]}),`.`]}),(0,b.jsxs)(`li`,{children:[`Use Anthropic-style names (`,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`); `,(0,b.jsx)(M,{children:`gpt-4o`}),` / `,(0,b.jsx)(M,{children:`gpt-4`}),` are the only OpenAI aliases.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(M,{children:`auto`}),` only works when the admin has enabled Auto Routing.`]}),(0,b.jsxs)(`li`,{children:[`A thinking suffix other than the configured one (default `,(0,b.jsx)(M,{children:`-thinking`}),`) is treated as part of the model name and fails.`]})]}),(0,b.jsx)(D,{id:`streaming`,children:`Streaming stalls or arrives all at once`}),(0,b.jsx)(k,{children:`The gateway flushes every SSE event. If you see nothing until the end, a proxy in between is buffering. Disable buffering for the API paths:`}),(0,b.jsx)(w,{lang:`nginx`,code:`location /v1/ {
    proxy_pass http://hekato:8080;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 600s;
    proxy_set_header Connection "";
    chunked_transfer_encoding on;
}`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[`Caddy: `,(0,b.jsxs)(M,{children:[`reverse_proxy hekato:8080 `,`{ flush_interval -1 }`]})]}),(0,b.jsx)(`li`,{children:`Cloudflare: streaming works on proxied hosts, but keep the request under the 100 s idle timeout on free plans or use long-lived thinking with care.`}),(0,b.jsxs)(`li`,{children:[`With curl, add `,(0,b.jsx)(M,{children:`-N`}),` to disable its own output buffering.`]})]}),(0,b.jsx)(D,{id:`cors`,children:`CORS`}),(0,b.jsxs)(k,{children:[`The gateway answers `,(0,b.jsx)(M,{children:`OPTIONS`}),` preflights with `,(0,b.jsx)(M,{children:`Access-Control-Allow-Origin: *`}),` and allows the `,(0,b.jsx)(M,{children:`Authorization`}),`,`,(0,b.jsx)(M,{children:`x-api-key`}),`, `,(0,b.jsx)(M,{children:`anthropic-version`}),` and `,(0,b.jsx)(M,{children:`anthropic-beta`}),` headers, so browser apps can call it directly. If you still get CORS errors, a proxy in front is stripping the headers or answering the preflight itself.`]}),(0,b.jsx)(T,{kind:`warning`,children:`Calling the gateway from browser code exposes your API key to anyone who opens DevTools. Use a dedicated key with a small quota.`}),(0,b.jsx)(D,{id:`thinking-output`,children:`Thinking text shows up in my answers`}),(0,b.jsxs)(k,{children:[`You are using a `,(0,b.jsx)(M,{children:`-thinking`}),` model with an OpenAI-style client, and the gateway is configured to surface thinking inline. Either drop the suffix or ask the admin to switch the thinking output format under Settings → Thinking Mode.`]})]})}function B(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Point Anthropic's Claude Code CLI at the gateway with two environment variables.`,children:`Claude Code`}),(0,b.jsx)(D,{id:`env`,children:`Environment variables`}),(0,b.jsx)(w,{lang:`bash`,code:`export ANTHROPIC_BASE_URL=${e}
export ANTHROPIC_AUTH_TOKEN=YOUR_API_KEY
# optional: pin models
export ANTHROPIC_MODEL=claude-sonnet-4.5
export ANTHROPIC_SMALL_FAST_MODEL=claude-haiku-4.5

claude`}),(0,b.jsxs)(k,{children:[(0,b.jsx)(M,{children:`ANTHROPIC_API_KEY`}),` works as well; `,(0,b.jsx)(M,{children:`ANTHROPIC_AUTH_TOKEN`}),` is preferred because it skips the interactive login prompt. Do not add `,(0,b.jsx)(M,{children:`/v1`}),` to the base URL.`]}),(0,b.jsx)(D,{id:`settings`,children:`Persist in ~/.claude/settings.json`}),(0,b.jsx)(w,{lang:`json`,code:`{
  "env": {
    "ANTHROPIC_BASE_URL": "${e}",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
    "ANTHROPIC_MODEL": "claude-sonnet-4.5",
    "ANTHROPIC_SMALL_FAST_MODEL": "claude-haiku-4.5"
  }
}`}),(0,b.jsxs)(k,{children:[`A project-level `,(0,b.jsx)(M,{children:`.claude/settings.json`}),` with the same `,(0,b.jsx)(M,{children:`env`}),` block overrides the global one, handy when only one repo should go through the gateway.`]}),(0,b.jsx)(D,{id:`thinking`,children:`Thinking`}),(0,b.jsxs)(k,{children:[`Set `,(0,b.jsx)(M,{children:`ANTHROPIC_MODEL=claude-sonnet-4.5-thinking`}),`, or use `,(0,b.jsx)(M,{children:`/model claude-sonnet-4.5-thinking`}),` inside the session. Claude Code's own "ultrathink" keywords also send an Anthropic `,(0,b.jsx)(M,{children:`thinking`}),` block, which the gateway honours.`]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsx)(w,{lang:`bash`,code:`claude -p "Reply with the single word OK"`}),(0,b.jsxs)(k,{children:[`Then open `,(0,b.jsx)(N,{href:`/usage`,children:`/usage`}),` with your key: the request counter should have moved. Telemetry calls to`,` `,(0,b.jsx)(M,{children:`/api/event_logging/batch`}),` are accepted and discarded by the gateway.`]})]})}function V(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`OpenAI's Codex CLI speaks the Responses API, which the gateway serves at /v1/responses.`,children:`Codex CLI`}),(0,b.jsx)(D,{id:`config`,children:`~/.codex/config.toml`}),(0,b.jsx)(w,{lang:`toml`,code:`model = "claude-sonnet-4.5"
model_provider = "hekato"

[model_providers.hekato]
name = "Hekato gateway"
base_url = "${e}/v1"
env_key = "HEKATO_API_KEY"
wire_api = "responses"`}),(0,b.jsx)(w,{lang:`bash`,code:`export HEKATO_API_KEY=YOUR_API_KEY
codex`}),(0,b.jsx)(D,{id:`chat-fallback`,children:`Chat Completions fallback`}),(0,b.jsx)(k,{children:`If a Codex version misbehaves with the Responses wire format, switch the provider to Chat Completions; everything else stays the same.`}),(0,b.jsx)(w,{lang:`toml`,code:`[model_providers.hekato]
name = "Hekato gateway (chat)"
base_url = "${e}/v1"
env_key = "HEKATO_API_KEY"
wire_api = "chat"`}),(0,b.jsx)(D,{id:`models`,children:`Models`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(M,{children:`model = "claude-sonnet-4.5"`}),` for everyday work, `,(0,b.jsx)(M,{children:`claude-opus-4.5`}),` for hard problems, `,(0,b.jsx)(M,{children:`claude-sonnet-4.5-thinking`}),` for extended reasoning.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(M,{children:`model = "auto"`}),` lets the gateway pick per request (if the admin enabled Auto Routing).`]}),(0,b.jsxs)(`li`,{children:[`Codex's `,(0,b.jsx)(M,{children:`model_reasoning_effort`}),` setting is ignored by the gateway; use the `,(0,b.jsx)(M,{children:`-thinking`}),` suffix instead.`]})]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsx)(w,{lang:`bash`,code:`codex exec "Reply with the single word OK"`})]})}function H(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Cursor can use an OpenAI-compatible base URL for its chat and composer features.`,children:`Cursor`}),(0,b.jsx)(D,{id:`setup`,children:`Setup`}),(0,b.jsxs)(j,{children:[(0,b.jsxs)(`li`,{children:[`Open `,(0,b.jsx)(`strong`,{children:`Cursor Settings → Models`}),`.`]}),(0,b.jsxs)(`li`,{children:[`Under `,(0,b.jsx)(`strong`,{children:`OpenAI API Key`}),`, paste `,(0,b.jsx)(M,{children:`YOUR_API_KEY`}),`.`]}),(0,b.jsxs)(`li`,{children:[`Enable `,(0,b.jsx)(`strong`,{children:`Override OpenAI Base URL`}),` and enter:`,(0,b.jsx)(w,{lang:`text`,code:`${e}/v1`})]}),(0,b.jsxs)(`li`,{children:[`Click `,(0,b.jsx)(`strong`,{children:`Verify`}),`. Cursor sends a test request through the gateway.`]}),(0,b.jsxs)(`li`,{children:[`Click `,(0,b.jsx)(`strong`,{children:`+ Add model`}),` and add the names you want to use, for example `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`, `,(0,b.jsx)(M,{children:`claude-opus-4.5`}),`,`,` `,(0,b.jsx)(M,{children:`claude-sonnet-4.5-thinking`}),` and `,(0,b.jsx)(M,{children:`auto`}),`. Untick the built-in models so Cursor cannot fall back to them.`]})]}),(0,b.jsx)(D,{id:`caveats`,children:`Caveats`}),(0,b.jsx)(T,{kind:`warning`,children:`Cursor always uses its own model IDs for some features (Tab completion, Apply, embeddings) regardless of the override, and those requests go to Cursor's servers, not the gateway. Only Chat / Composer with a custom model name is routed through your base URL.`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[`If you pick a built-in name like `,(0,b.jsx)(M,{children:`gpt-4o`}),`, the gateway aliases it to `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`, so it still works.`]}),(0,b.jsx)(`li`,{children:`The override is global: turn it off to go back to Cursor's own models. Cursor may prompt you to disable it when using some features.`}),(0,b.jsx)(`li`,{children:`Anthropic key / base URL overrides in Cursor are not used; keep everything under the OpenAI section.`})]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsxs)(k,{children:[`Open Chat, select `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`, ask a question, then check `,(0,b.jsx)(N,{href:`/usage`,children:`/usage`}),`.`]})]})}function U(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Both VS Code agents accept a custom Anthropic or OpenAI-compatible endpoint.`,children:`Cline / Roo Code`}),(0,b.jsx)(D,{id:`anthropic`,children:`Option A: Anthropic provider (recommended)`}),(0,b.jsx)(k,{children:`Native tool use, thinking blocks and prompt caching are all preserved on this path.`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`API Provider`}),`: Anthropic`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Anthropic API Key`}),`: `,(0,b.jsx)(M,{children:`YOUR_API_KEY`})]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Use custom base URL`}),`: enabled`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Base URL`}),`: `,(0,b.jsx)(M,{children:e}),` (no `,(0,b.jsx)(M,{children:`/v1`}),`)`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Model`}),`: `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),`. If the model picker only lists dated IDs, pick any Sonnet 4.5 entry; the gateway normalizes dated names.`]})]}),(0,b.jsx)(D,{id:`openai`,children:`Option B: OpenAI Compatible provider`}),(0,b.jsxs)(A,{children:[(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`API Provider`}),`: OpenAI Compatible`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Base URL`}),`: `,(0,b.jsxs)(M,{children:[e,`/v1`]})]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`API Key`}),`: `,(0,b.jsx)(M,{children:`YOUR_API_KEY`})]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Model ID`}),`: `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),` (or `,(0,b.jsx)(M,{children:`auto`}),`, `,(0,b.jsx)(M,{children:`claude-sonnet-4.5-thinking`}),`)`]}),(0,b.jsxs)(`li`,{children:[`Roo Code: leave `,(0,b.jsx)(`strong`,{children:`Enable streaming`}),` on; set context window to 200000 if asked.`]})]}),(0,b.jsx)(D,{id:`thinking`,children:`Thinking`}),(0,b.jsxs)(k,{children:[`With the Anthropic provider, turning on "Extended thinking" in the agent settings sends a `,(0,b.jsx)(M,{children:`thinking`}),` block which the gateway honours. With the OpenAI-compatible provider, use the `,(0,b.jsx)(M,{children:`-thinking`}),` model name instead.`]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsxs)(k,{children:[`Start a task such as "list the files in this workspace". The first response confirms the route; the request count on`,` `,(0,b.jsx)(N,{href:`/usage`,children:`/usage`}),` confirms the key.`]})]})}function W(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Continue reads models from ~/.continue/config.yaml. Either provider type works.`,children:`Continue`}),(0,b.jsx)(D,{id:`anthropic`,children:`Anthropic provider`}),(0,b.jsx)(w,{lang:`yaml`,code:`name: Local Assistant
version: 1.0.0
schema: v1

models:
  - name: Claude Sonnet 4.5 (Hekato)
    provider: anthropic
    model: claude-sonnet-4.5
    apiBase: ${e}
    apiKey: YOUR_API_KEY
    roles: [chat, edit, apply]

  - name: Claude Haiku 4.5 (Hekato)
    provider: anthropic
    model: claude-haiku-4.5
    apiBase: ${e}
    apiKey: YOUR_API_KEY
    roles: [autocomplete]`}),(0,b.jsx)(D,{id:`openai`,children:`OpenAI-compatible provider`}),(0,b.jsx)(w,{lang:`yaml`,code:`models:
  - name: Claude Sonnet 4.5 (Hekato, OpenAI wire)
    provider: openai
    model: claude-sonnet-4.5
    apiBase: ${e}/v1
    apiKey: YOUR_API_KEY
    roles: [chat, edit, apply]`}),(0,b.jsx)(D,{id:`legacy`,children:`Legacy config.json`}),(0,b.jsx)(w,{lang:`json`,code:`{
  "models": [
    {
      "title": "Claude Sonnet 4.5 (Hekato)",
      "provider": "anthropic",
      "model": "claude-sonnet-4.5",
      "apiBase": "${e}",
      "apiKey": "YOUR_API_KEY"
    }
  ]
}`}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsxs)(k,{children:[`Reload the window, pick the model in the Continue sidebar, send "Reply with OK", then check `,(0,b.jsx)(N,{href:`/usage`,children:`/usage`}),`.`]})]})}function G(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Aider uses LiteLLM under the hood; prefix the model with the provider you want to speak.`,children:`Aider`}),(0,b.jsx)(D,{id:`openai`,children:`OpenAI-compatible`}),(0,b.jsx)(w,{lang:`bash`,code:`export OPENAI_API_BASE=${e}/v1
export OPENAI_API_KEY=YOUR_API_KEY

aider --model openai/claude-sonnet-4.5`}),(0,b.jsx)(D,{id:`anthropic`,children:`Anthropic`}),(0,b.jsx)(w,{lang:`bash`,code:`export ANTHROPIC_API_BASE=${e}
export ANTHROPIC_API_KEY=YOUR_API_KEY

aider --model anthropic/claude-sonnet-4.5`}),(0,b.jsx)(D,{id:`config`,children:`.aider.conf.yml`}),(0,b.jsx)(w,{lang:`yaml`,code:`model: openai/claude-sonnet-4.5
weak-model: openai/claude-haiku-4.5
openai-api-base: ${e}/v1
openai-api-key: YOUR_API_KEY`}),(0,b.jsxs)(T,{children:[`Aider warns "Unknown model" for names it has no metadata for. That is harmless; add a `,(0,b.jsx)(M,{children:`.aider.model.metadata.json`}),` with`,` `,(0,b.jsx)(M,{children:`max_input_tokens`}),` if you want the warning gone.`]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsx)(w,{lang:`bash`,code:`aider --model openai/claude-sonnet-4.5 --message "Reply with the single word OK" --no-git`})]})}function K(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Both chat front-ends treat the gateway as an OpenAI-compatible provider.`,children:`Open WebUI / LibreChat`}),(0,b.jsx)(D,{id:`open-webui`,children:`Open WebUI`}),(0,b.jsxs)(j,{children:[(0,b.jsxs)(`li`,{children:[`Log in as admin and open `,(0,b.jsx)(`strong`,{children:`Admin Panel → Settings → Connections`}),`.`]}),(0,b.jsxs)(`li`,{children:[`Under `,(0,b.jsx)(`strong`,{children:`OpenAI API`}),`, click `,(0,b.jsx)(`strong`,{children:`+`}),` to add a connection.`]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`URL`}),`: `,(0,b.jsxs)(M,{children:[e,`/v1`]})]}),(0,b.jsxs)(`li`,{children:[(0,b.jsx)(`strong`,{children:`Key`}),`: `,(0,b.jsx)(M,{children:`YOUR_API_KEY`})]}),(0,b.jsxs)(`li`,{children:[`Save. The model list is fetched from `,(0,b.jsx)(M,{children:`/v1/models`}),`; the Claude models, `,(0,b.jsx)(M,{children:`-thinking`}),` variants and `,(0,b.jsx)(M,{children:`auto`}),` appear in the model picker.`]})]}),(0,b.jsx)(k,{children:`Or via environment variables when you run the container:`}),(0,b.jsx)(w,{lang:`bash`,code:`docker run -d -p 3000:8080 \\
  -e OPENAI_API_BASE_URL=${e}/v1 \\
  -e OPENAI_API_KEY=YOUR_API_KEY \\
  -v open-webui:/app/backend/data \\
  --name open-webui ghcr.io/open-webui/open-webui:main`}),(0,b.jsx)(D,{id:`librechat`,children:`LibreChat`}),(0,b.jsxs)(k,{children:[`Add a custom endpoint in `,(0,b.jsx)(M,{children:`librechat.yaml`}),`:`]}),(0,b.jsx)(w,{lang:`yaml`,code:`version: 1.2.1
endpoints:
  custom:
    - name: "Hekato"
      apiKey: "\${HEKATO_API_KEY}"
      baseURL: "${e}/v1"
      models:
        default: ["claude-sonnet-4.5", "claude-opus-4.5", "claude-haiku-4.5", "claude-sonnet-4.5-thinking", "auto"]
        fetch: true
      titleConvo: true
      titleModel: "claude-haiku-4.5"
      modelDisplayLabel: "Hekato"
      dropParams: ["stop", "user", "frequency_penalty", "presence_penalty"]`}),(0,b.jsx)(w,{lang:`bash`,code:`# .env
HEKATO_API_KEY=YOUR_API_KEY`}),(0,b.jsxs)(T,{children:[`LibreChat can also use its native Anthropic endpoint: set `,(0,b.jsx)(M,{children:`ANTHROPIC_API_KEY=YOUR_API_KEY`}),` and`,` `,(0,b.jsxs)(M,{children:[`ANTHROPIC_REVERSE_PROXY=`,e,`/v1/messages`]}),` in `,(0,b.jsx)(M,{children:`.env`}),`. This path keeps thinking blocks and prompt caching.`]}),(0,b.jsx)(D,{id:`verify`,children:`Verify`}),(0,b.jsxs)(k,{children:[`Pick `,(0,b.jsx)(M,{children:`claude-sonnet-4.5`}),` in the model selector, send a message, then confirm the request count on `,(0,b.jsx)(N,{href:`/usage`,children:`/usage`}),`.`]})]})}function q(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Copy-paste reference for every endpoint using nothing but curl.`,children:`curl reference`}),(0,b.jsx)(D,{id:`non-stream`,children:`Non-streaming`}),(0,b.jsx)(w,{lang:`bash`,code:`# Anthropic
curl ${e}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":256,"messages":[{"role":"user","content":"Hello"}]}'

# OpenAI Chat
curl ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"Hello"}]}'

# OpenAI Responses
curl ${e}/v1/responses \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","input":"Hello"}'`}),(0,b.jsx)(D,{id:`stream`,children:`Streaming`}),(0,b.jsx)(w,{lang:`bash`,code:`# Anthropic SSE
curl -N ${e}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"Count to five."}]}'

# OpenAI SSE
curl -N ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","stream":true,"messages":[{"role":"user","content":"Count to five."}]}'`}),(0,b.jsx)(D,{id:`tools`,children:`Tool use`}),(0,b.jsx)(w,{lang:`bash`,code:`# Anthropic tools
curl ${e}/v1/messages \\
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
curl ${e}/v1/chat/completions \\
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
  }'`}),(0,b.jsx)(D,{id:`images`,children:`Images`}),(0,b.jsx)(w,{lang:`bash`,code:`IMG=$(base64 -i photo.jpg | tr -d '\\n')

# Anthropic
curl ${e}/v1/messages \\
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
curl ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "{
    \\"model\\": \\"claude-sonnet-4.5\\",
    \\"messages\\": [{\\"role\\":\\"user\\",\\"content\\":[
      {\\"type\\":\\"image_url\\",\\"image_url\\":{\\"url\\":\\"data:image/jpeg;base64,$IMG\\"}},
      {\\"type\\":\\"text\\",\\"text\\":\\"Describe this image.\\"}
    ]}]
  }"`}),(0,b.jsx)(D,{id:`thinking`,children:`Thinking`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":2048},"messages":[{"role":"user","content":"Prove that sqrt(2) is irrational."}]}'`}),(0,b.jsx)(D,{id:`count-tokens`,children:`Count tokens`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/messages/count_tokens \\
  -H "x-api-key: YOUR_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"How many tokens is this?"}]}'`}),(0,b.jsx)(D,{id:`models`,children:`Models`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/models | jq -r '.data[].id'`}),(0,b.jsx)(D,{id:`usage`,children:`Usage`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/v1/usage -H "Authorization: Bearer YOUR_API_KEY" | jq`}),(0,b.jsx)(D,{id:`health`,children:`Health`}),(0,b.jsx)(w,{lang:`bash`,code:`curl ${e}/health`}),(0,b.jsx)(D,{id:`auto`,children:`Auto routing headers`}),(0,b.jsx)(w,{lang:`bash`,code:`curl -si ${e}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello"}]}' | grep -i '^x-hekato'`})]})}function J(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`The official OpenAI SDKs only need a base_url and an api_key.`,children:`OpenAI SDK`}),(0,b.jsx)(D,{id:`python`,children:`Python`}),(0,b.jsx)(w,{lang:`bash`,code:`pip install openai`}),(0,b.jsx)(w,{lang:`python`,code:`from openai import OpenAI

client = OpenAI(base_url="${e}/v1", api_key="YOUR_API_KEY")

resp = client.chat.completions.create(
    model="claude-sonnet-4.5",
    messages=[{"role": "user", "content": "Say hello in one sentence."}],
)
print(resp.choices[0].message.content)`}),(0,b.jsx)(O,{id:`python-stream`,children:`Streaming`}),(0,b.jsx)(w,{lang:`python`,code:`stream = client.chat.completions.create(
    model="claude-sonnet-4.5",
    messages=[{"role": "user", "content": "Count to five."}],
    stream=True,
)
for chunk in stream:
    delta = chunk.choices[0].delta.content
    if delta:
        print(delta, end="", flush=True)`}),(0,b.jsx)(O,{id:`python-responses`,children:`Responses API`}),(0,b.jsx)(w,{lang:`python`,code:`resp = client.responses.create(
    model="claude-sonnet-4.5",
    instructions="Be brief.",
    input="What is an AI gateway?",
)
print(resp.output_text)`}),(0,b.jsx)(D,{id:`node`,children:`Node / TypeScript`}),(0,b.jsx)(w,{lang:`bash`,code:`npm install openai`}),(0,b.jsx)(w,{lang:`typescript`,code:`import OpenAI from 'openai'

const client = new OpenAI({ baseURL: '${e}/v1', apiKey: 'YOUR_API_KEY' })

const resp = await client.chat.completions.create({
  model: 'claude-sonnet-4.5',
  messages: [{ role: 'user', content: 'Say hello in one sentence.' }],
})
console.log(resp.choices[0].message.content)`}),(0,b.jsx)(O,{id:`node-stream`,children:`Streaming`}),(0,b.jsx)(w,{lang:`typescript`,code:`const stream = await client.chat.completions.create({
  model: 'claude-sonnet-4.5',
  messages: [{ role: 'user', content: 'Count to five.' }],
  stream: true,
})
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '')
}`}),(0,b.jsx)(O,{id:`node-responses`,children:`Responses API`}),(0,b.jsx)(w,{lang:`typescript`,code:`const r = await client.responses.create({
  model: 'claude-sonnet-4.5',
  instructions: 'Be brief.',
  input: 'What is an AI gateway?',
})
console.log(r.output_text)`}),(0,b.jsx)(D,{id:`env`,children:`Environment variables`}),(0,b.jsx)(k,{children:`Both SDKs also read these, so you can leave the constructor empty:`}),(0,b.jsx)(w,{lang:`bash`,code:`export OPENAI_BASE_URL=${e}/v1
export OPENAI_API_KEY=YOUR_API_KEY`}),(0,b.jsxs)(T,{children:[`Thinking: use `,(0,b.jsx)(M,{children:`model="claude-sonnet-4.5-thinking"`}),`. The OpenAI wire format has no thinking parameter, so the suffix is the switch.`]})]})}function Y(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`The official Anthropic SDKs work unchanged; pass the gateway as base_url (no /v1).`,children:`Anthropic SDK`}),(0,b.jsx)(D,{id:`python`,children:`Python`}),(0,b.jsx)(w,{lang:`bash`,code:`pip install anthropic`}),(0,b.jsx)(w,{lang:`python`,code:`import anthropic

client = anthropic.Anthropic(base_url="${e}", api_key="YOUR_API_KEY")

msg = client.messages.create(
    model="claude-sonnet-4.5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Say hello in one sentence."}],
)
print(msg.content[0].text)`}),(0,b.jsx)(O,{id:`python-stream`,children:`Streaming`}),(0,b.jsx)(w,{lang:`python`,code:`with client.messages.stream(
    model="claude-sonnet-4.5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Count to five."}],
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)`}),(0,b.jsx)(O,{id:`python-thinking`,children:`Thinking block`}),(0,b.jsx)(w,{lang:`python`,code:`msg = client.messages.create(
    model="claude-sonnet-4.5",
    max_tokens=4096,
    thinking={"type": "enabled", "budget_tokens": 2048},
    messages=[{"role": "user", "content": "Prove that sqrt(2) is irrational."}],
)
for block in msg.content:
    if block.type == "thinking":
        print("[thinking]", block.thinking[:200], "...")
    elif block.type == "text":
        print(block.text)`}),(0,b.jsx)(D,{id:`node`,children:`Node / TypeScript`}),(0,b.jsx)(w,{lang:`bash`,code:`npm install @anthropic-ai/sdk`}),(0,b.jsx)(w,{lang:`typescript`,code:`import Anthropic from '@anthropic-ai/sdk'

const client = new Anthropic({ baseURL: '${e}', apiKey: 'YOUR_API_KEY' })

const msg = await client.messages.create({
  model: 'claude-sonnet-4.5',
  max_tokens: 256,
  messages: [{ role: 'user', content: 'Say hello in one sentence.' }],
})
console.log(msg.content[0].type === 'text' ? msg.content[0].text : msg.content)`}),(0,b.jsx)(O,{id:`node-stream`,children:`Streaming`}),(0,b.jsx)(w,{lang:`typescript`,code:`const stream = client.messages.stream({
  model: 'claude-sonnet-4.5',
  max_tokens: 256,
  messages: [{ role: 'user', content: 'Count to five.' }],
})
stream.on('text', (t) => process.stdout.write(t))
await stream.finalMessage()`}),(0,b.jsx)(O,{id:`node-thinking`,children:`Thinking block`}),(0,b.jsx)(w,{lang:`typescript`,code:`const msg = await client.messages.create({
  model: 'claude-sonnet-4.5',
  max_tokens: 4096,
  thinking: { type: 'enabled', budget_tokens: 2048 },
  messages: [{ role: 'user', content: 'Prove that sqrt(2) is irrational.' }],
})`}),(0,b.jsx)(D,{id:`env`,children:`Environment variables`}),(0,b.jsx)(w,{lang:`bash`,code:`export ANTHROPIC_BASE_URL=${e}
export ANTHROPIC_API_KEY=YOUR_API_KEY`}),(0,b.jsxs)(T,{children:[`Token counting: `,(0,b.jsx)(M,{children:`client.messages.count_tokens(...)`}),` hits `,(0,b.jsx)(M,{children:`/v1/messages/count_tokens`}),` and returns a local estimate.`]})]})}function te(){let{base:e}=C();return(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(E,{lead:`Use either the OpenAI or the Anthropic chat model class; both accept a custom base URL.`,children:`LangChain`}),(0,b.jsx)(D,{id:`openai`,children:`ChatOpenAI`}),(0,b.jsx)(w,{lang:`bash`,code:`pip install langchain-openai`}),(0,b.jsx)(w,{lang:`python`,code:`from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="${e}/v1",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    temperature=0,
)
print(llm.invoke("Say hello in one sentence.").content)

# streaming
for chunk in llm.stream("Count to five."):
    print(chunk.content, end="", flush=True)`}),(0,b.jsx)(D,{id:`anthropic`,children:`ChatAnthropic`}),(0,b.jsx)(w,{lang:`bash`,code:`pip install langchain-anthropic`}),(0,b.jsx)(w,{lang:`python`,code:`from langchain_anthropic import ChatAnthropic

llm = ChatAnthropic(
    base_url="${e}",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    max_tokens=1024,
)
print(llm.invoke("Say hello in one sentence.").content)

# extended thinking
thinking_llm = ChatAnthropic(
    base_url="${e}",
    api_key="YOUR_API_KEY",
    model="claude-sonnet-4.5",
    max_tokens=4096,
    thinking={"type": "enabled", "budget_tokens": 2048},
)`}),(0,b.jsx)(D,{id:`tools`,children:`Tool calling`}),(0,b.jsx)(k,{children:`Tool binding works the same on both classes; the gateway translates tool schemas and tool results for the upstream.`}),(0,b.jsx)(w,{lang:`python`,code:`from langchain_core.tools import tool

@tool
def add(a: int, b: int) -> int:
    """Add two integers."""
    return a + b

agent = llm.bind_tools([add])
print(agent.invoke("What is 2 + 3? Use the tool.").tool_calls)`}),(0,b.jsxs)(T,{children:[`JavaScript: `,(0,b.jsx)(M,{children:`@langchain/openai`}),` takes `,(0,b.jsx)(M,{children:`configuration: { baseURL }`}),`; `,(0,b.jsx)(M,{children:`@langchain/anthropic`}),` takes`,` `,(0,b.jsx)(M,{children:`clientOptions: { baseURL }`}),`.`]})]})}var X=[{key:`docs.group.overview`,pages:[{slug:`getting-started`,title:`Getting started`,component:F},{slug:`endpoints`,title:`Endpoints`,component:I},{slug:`models`,title:`Models & thinking`,component:L},{slug:`limits`,title:`Limits & usage`,component:R}]},{key:`docs.group.agents`,pages:[{slug:`claude-code`,title:`Claude Code`,component:B},{slug:`codex-cli`,title:`Codex CLI`,component:V},{slug:`cursor`,title:`Cursor`,component:H},{slug:`cline-roo`,title:`Cline / Roo Code`,component:U},{slug:`continue`,title:`Continue`,component:W},{slug:`aider`,title:`Aider`,component:G}]},{key:`docs.group.sdks`,pages:[{slug:`openai-sdk`,title:`OpenAI SDK`,component:J},{slug:`anthropic-sdk`,title:`Anthropic SDK`,component:Y},{slug:`langchain`,title:`LangChain`,component:te}]},{key:`docs.group.apps`,pages:[{slug:`open-webui-librechat`,title:`Open WebUI / LibreChat`,component:K},{slug:`curl`,title:`curl reference`,component:q}]},{key:`docs.group.help`,pages:[{slug:`troubleshooting`,title:`Troubleshooting`,component:z}]}],Z=X.flatMap(e=>e.pages),ne=`getting-started`,Q=`hekato_docs_key`;function $(){let e=window.location.pathname.match(/^\/docs\/?([^/]*)/)?.[1]??``;return Z.some(t=>t.slug===e)?e:ne}function re(){let{t:e,lang:t,setLang:n}=s(),{theme:r,toggle:i}=f(),[u,p]=(0,y.useState)($),[m,h]=(0,y.useState)(()=>sessionStorage.getItem(Q)??``),[g,_]=(0,y.useState)([]),v=window.location.origin,x=Z.find(e=>e.slug===u)??Z[0],C=e=>{e!==u&&(history.pushState(null,``,`/docs/`+e),p(e),window.scrollTo({top:0}))};(0,y.useEffect)(()=>{let e=()=>p($());return window.addEventListener(`popstate`,e),()=>window.removeEventListener(`popstate`,e)},[]),(0,y.useEffect)(()=>{let e=e=>{let t=e.target.closest(`a[href^="/docs/"]`);if(!t||e.metaKey||e.ctrlKey)return;let[n,r]=(t.getAttribute(`href`)??``).split(`#`),i=n.replace(`/docs/`,``);Z.some(e=>e.slug===i)&&(e.preventDefault(),C(i),r&&setTimeout(()=>document.getElementById(r)?.scrollIntoView(),0))};return document.addEventListener(`click`,e),()=>document.removeEventListener(`click`,e)}),(0,y.useEffect)(()=>{sessionStorage.setItem(Q,m)},[m]),(0,y.useEffect)(()=>{let e=Array.from(document.querySelectorAll(`article h2[id], article h3[id]`));_(e.map(e=>({id:e.id,text:e.textContent??``,level:e.tagName===`H2`?2:3}))),document.title=`${x.title} · Hekato Docs`},[u,x.title]);let w=(0,y.useMemo)(()=>({base:v,apiKey:m.trim()}),[v,m]),T=x.component,E=(0,b.jsx)(`nav`,{className:`space-y-5 text-sm`,children:X.map(t=>(0,b.jsxs)(`div`,{children:[(0,b.jsx)(`div`,{className:`mb-1.5 px-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground`,children:e(t.key)}),(0,b.jsx)(`ul`,{className:`space-y-0.5`,children:t.pages.map(e=>(0,b.jsx)(`li`,{children:(0,b.jsx)(`a`,{href:`/docs/`+e.slug,onClick:t=>{t.preventDefault(),C(e.slug)},className:o(`block rounded-md px-2 py-1.5 transition-colors hover:bg-muted`,e.slug===u?`bg-muted font-medium text-foreground`:`text-muted-foreground`),children:e.title})},e.slug))})]},t.key))});return(0,b.jsx)(S.Provider,{value:w,children:(0,b.jsxs)(`div`,{className:`flex min-h-svh flex-col bg-background`,children:[(0,b.jsx)(`header`,{className:`sticky top-0 z-30 border-b bg-background/90 backdrop-blur`,children:(0,b.jsxs)(`div`,{className:`mx-auto flex h-14 max-w-screen-2xl items-center justify-between gap-3 px-4`,children:[(0,b.jsxs)(`a`,{href:`/docs`,className:`flex items-center gap-2`,children:[(0,b.jsx)(`div`,{className:`flex size-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground`,children:`H`}),(0,b.jsx)(`span`,{className:`text-sm font-semibold`,children:e(`docs.title`)})]}),(0,b.jsxs)(`div`,{className:`flex items-center gap-1`,children:[(0,b.jsx)(a,{variant:`ghost`,size:`sm`,render:(0,b.jsx)(`a`,{href:`/usage`}),children:e(`docs.usageLink`)}),(0,b.jsx)(a,{variant:`ghost`,size:`sm`,render:(0,b.jsx)(`a`,{href:`/admin`}),children:e(`docs.adminLink`)}),(0,b.jsx)(a,{variant:`ghost`,size:`icon`,onClick:()=>n(t===`en`?`zh`:`en`),"aria-label":e(`header.language`),children:(0,b.jsx)(l,{className:`size-4`})}),(0,b.jsx)(a,{variant:`ghost`,size:`icon`,onClick:i,"aria-label":e(`header.theme`),children:r===`dark`?(0,b.jsx)(ee,{className:`size-4`}):(0,b.jsx)(c,{className:`size-4`})})]})]})}),(0,b.jsxs)(`div`,{className:`mx-auto flex w-full max-w-screen-2xl flex-1 gap-8 px-4`,children:[(0,b.jsx)(`aside`,{className:`sticky top-14 hidden h-[calc(100svh-3.5rem)] w-56 shrink-0 overflow-y-auto py-6 md:block`,children:E}),(0,b.jsxs)(`main`,{className:`min-w-0 flex-1 py-6`,children:[(0,b.jsxs)(`div`,{className:`mb-6 flex flex-col gap-3 rounded-lg border bg-muted/30 p-3 sm:flex-row sm:items-center`,children:[(0,b.jsx)(`label`,{htmlFor:`docs-key`,className:`shrink-0 text-sm font-medium`,children:e(`docs.yourKey`)}),(0,b.jsx)(d,{id:`docs-key`,type:`password`,autoComplete:`off`,placeholder:e(`docs.keyPlaceholder`),value:m,onChange:e=>h(e.target.value),className:`font-mono`}),(0,b.jsx)(`span`,{className:`shrink-0 text-xs text-muted-foreground`,children:e(`docs.keyHint`)})]}),(0,b.jsx)(`select`,{className:`mb-6 h-9 w-full rounded-lg border bg-background px-2 text-sm md:hidden`,value:u,onChange:e=>C(e.target.value),"aria-label":e(`docs.navigation`),children:X.map(t=>(0,b.jsx)(`optgroup`,{label:e(t.key),children:t.pages.map(e=>(0,b.jsx)(`option`,{value:e.slug,children:e.title},e.slug))},t.key))}),(0,b.jsx)(`article`,{className:`mx-auto max-w-3xl text-[15px]`,children:(0,b.jsx)(T,{})},u)]}),(0,b.jsx)(`aside`,{className:`sticky top-14 hidden h-[calc(100svh-3.5rem)] w-52 shrink-0 overflow-y-auto py-6 xl:block`,children:g.length>0&&(0,b.jsxs)(b.Fragment,{children:[(0,b.jsx)(`div`,{className:`mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground`,children:e(`docs.onThisPage`)}),(0,b.jsx)(`ul`,{className:`space-y-1 border-l text-[13px]`,children:g.map(e=>(0,b.jsx)(`li`,{children:(0,b.jsx)(`a`,{href:`#`+e.id,className:o(`block border-l-2 border-transparent py-0.5 text-muted-foreground hover:text-foreground`,e.level===2?`pl-3`:`pl-6`),children:e.text})},e.id))})]})})]})]})})}export{re as DocsPage};