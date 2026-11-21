# Datacircle tools for Fantasy

Find the best GTM APIs. We benchmark all the GTM APIs and we put the results on our website: https://datacircle.dev

This Go module gives [Fantasy](https://github.com/charmbracelet/fantasy) agents four tools on Datacircle's open API, https://api.datacircle.dev/v1. You need no key and no account.

| Tool | What the tool gives | Call |
| --- | --- | --- |
| `datacircle_list_apis` | Every GTM API on Datacircle, with the API's slug, number of plans, number of providers and best plan | `GET /apis` |
| `datacircle_best_plans` | One API's plans, sorted by score out of 100, best first. Each plan comes with a rank and a price per 1,000 calls | `GET /apis/{slug}/best` |
| `datacircle_search_plans` | Plans that match a provider or plan name, on all APIs or on one API. Sort by score, provider or price | `GET /plans` |
| `datacircle_provider` | The APIs a provider sells, and the provider's rank on each API | `GET /providers/{slug}` |

## Install

```bash
go get github.com/waynehamadi/fantasy-datacircle
```

It needs Fantasy v0.1.0 or later.

## Use the tools

```go
import (
	"charm.land/fantasy"
	datacircle "github.com/waynehamadi/fantasy-datacircle"
)

agent := fantasy.NewAgent(model, fantasy.WithTools(datacircle.NewTools(nil)...))
result, err := agent.Generate(ctx, fantasy.AgentCall{Prompt: "Find the 3 best providers for the email-finder API."})
fmt.Println(result.Response.Content.Text())
```

`agent.Stream` calls the same tools. Each tool has its own constructor: `NewListAPIsTool`, `NewBestPlansTool`, `NewSearchPlansTool` and `NewProviderTool`. Give a model only the ones it needs. Each one is a `fantasy.AgentTool` made by `fantasy.NewAgentTool`, with a typed input (`ListAPIsInput`, `BestPlansInput`, `SearchPlansInput`, `ProviderInput`), so you can also call it yourself:

```go
res, err := datacircle.NewBestPlansTool(nil).Run(ctx, fantasy.ToolCall{Input: `{"slug":"email-finder","limit":3}`})
fmt.Println(res.Content)
```

Each tool gives back a JSON object as its content. Each plan comes with its rank, provider, plan, score, price per 1,000 (`perThousand`, in dollars) and its page on datacircle.dev. That page lists every field. `datacircle_provider` starts with a sentence of its counts: the APIs where the provider has plans, those where it has a rank, and its best rank.

When the API gives no data, like for a slug it doesn't know, the tool's response is an error (`IsError`) with the API's own words and what to call instead. When the API can't be reached, the error says so. Fantasy shows the model the error, so the model can call again, and the run goes on. A canceled context is the tool's error.

`datacircle.Config` sets the `HTTPClient` the tools use (a timeout, a proxy). `nil` uses `http.DefaultClient`.

## Network

Each tool call makes one HTTPS GET request to https://api.datacircle.dev/v1 and nothing else. The request holds only the tool's own input: no key, no config, nothing from your chats. The API takes up to 60 calls a minute from one IP address. The tool waits once, for a minute at most, when the API asks the tool to wait. The module depends only on Fantasy and `golang.org/x/text`, and stores nothing.

## Links

- Datacircle: https://datacircle.dev
- The API: https://datacircle.dev/api
- The API's spec: https://api.datacircle.dev/v1/openapi.json
- What each key means: https://datacircle.dev/fields

MIT license.
