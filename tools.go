package datacircle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"charm.land/fantasy"
)

const (
	ListAPIsName    = "datacircle_list_apis"
	BestPlansName   = "datacircle_best_plans"
	SearchPlansName = "datacircle_search_plans"
	ProviderName    = "datacircle_provider"

	listAPIsAbout    = "Lists the GTM APIs on Datacircle's sheet (https://datacircle.dev), with each API's slug, how many plans and providers it has and its best plan. Call it for an API's slug you don't know: datacircle_best_plans takes a slug, like email-finder. A provider's plans are in datacircle_search_plans and datacircle_provider."
	bestPlansAbout   = "Gives an API's plans best first by their score out of 100 on Datacircle's benchmark (https://datacircle.dev), each with its rank, provider, plan, $ per 1,000 calls and its page. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	searchPlansAbout = "Searches every plan on Datacircle's sheet (https://datacircle.dev), or one API's, by provider or plan name, sorted by score, provider or price. Gives how many were found and up to limit of them. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	providerAbout    = "Gives the APIs one provider sells and its rank on each on Datacircle's benchmark (https://datacircle.dev), with its best plan there, its best rank first. total counts the APIs it sells, ranked those where it has a rank (its place among the API's providers, 1 the best)."
)

// ListAPIsInput is datacircle_list_apis' input.
type ListAPIsInput struct {
	Query string `json:"query,omitempty" description:"Words of the API's name or slug, like email or linkedin profile: the APIs that hold the most of them come first. The sheet's order when left out"`
	Limit Count  `json:"limit,omitempty" description:"How many APIs to give, 1 to 200. 10 when left out: the sheet has about 150"`
}

// BestPlansInput is datacircle_best_plans' input.
type BestPlansInput struct {
	Slug  string `json:"slug" description:"The API's slug, like email-finder or linkedin-profile, as datacircle_list_apis gives it"`
	Limit Count  `json:"limit,omitempty" description:"How many plans to give, 1 to 50. 10 when left out"`
}

// SearchPlansInput is datacircle_search_plans' input.
type SearchPlansInput struct {
	API    string `json:"api,omitempty" description:"Only this API's plans: its name or slug, like email-finder. A provider's name goes in query, never here. Every API when left out"`
	Query  string `json:"query,omitempty" description:"Words that must all appear in the provider's or plan's name, like bright data"`
	Sort   string `json:"sort,omitempty" enum:"score,provider,price" description:"score (best first, the default), provider (A to Z) or price ($ per 1,000, lowest first)"`
	Limit  Count  `json:"limit,omitempty" description:"How many plans to give, 1 to 50. 10 when left out"`
	Offset Count  `json:"offset,omitempty" description:"How many found plans to skip first. 0 when left out"`
}

// ProviderInput is datacircle_provider's input.
type ProviderInput struct {
	Slug  string `json:"slug" description:"The provider's slug, like bright-data, or its name, like Bright Data: the name is made a slug"`
	Limit Count  `json:"limit,omitempty" description:"How many of its APIs to give, its best rank first, 1 to 200. 10 when left out"`
}

// Count is a whole number a model may send as a number, as text ("3") or as null, which is 0: left out.
type Count int

func (n *Count) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case float64:
		*n = Count(v)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		*n = Count(i)
	case nil:
		*n = 0
	default:
		return errors.New("a whole number")
	}
	return nil
}

// NewTools gives the four tools, datacircle_list_apis, datacircle_best_plans, datacircle_search_plans and
// datacircle_provider, for fantasy.WithTools.
func NewTools(cfg *Config) []fantasy.AgentTool {
	c := newClient(cfg)
	return []fantasy.AgentTool{listAPIsTool(c), bestPlansTool(c), searchPlansTool(c), providerTool(c)}
}

// NewListAPIsTool gives datacircle_list_apis: GET /apis.
func NewListAPIsTool(cfg *Config) fantasy.AgentTool { return listAPIsTool(newClient(cfg)) }

// NewBestPlansTool gives datacircle_best_plans: GET /apis/{slug}/best.
func NewBestPlansTool(cfg *Config) fantasy.AgentTool { return bestPlansTool(newClient(cfg)) }

// NewSearchPlansTool gives datacircle_search_plans: GET /plans.
func NewSearchPlansTool(cfg *Config) fantasy.AgentTool { return searchPlansTool(newClient(cfg)) }

// NewProviderTool gives datacircle_provider: GET /providers/{slug}.
func NewProviderTool(cfg *Config) fantasy.AgentTool { return providerTool(newClient(cfg)) }

// answer gives a call's result as the tool's response: its JSON, or an error response with the API's words or the
// failure's, which the model reads and can call again on. Only a canceled call is the tool's error.
func answer(ctx context.Context, v map[string]any, err error) (fantasy.ToolResponse, error) {
	if err != nil {
		if ctx.Err() != nil {
			return fantasy.ToolResponse{}, ctx.Err()
		}
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	content, err := json.Marshal(v)
	if err != nil {
		return fantasy.NewTextErrorResponse("datacircle: " + err.Error()), nil
	}
	return fantasy.NewTextResponse(string(content)), nil
}

// advice is a message for the model, like what to send instead: an error response it reads.
type advice string

func (a advice) Error() string { return string(a) }

func listAPIsTool(c *client) fantasy.AgentTool {
	return fantasy.NewAgentTool(ListAPIsName, listAPIsAbout,
		func(ctx context.Context, in ListAPIsInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			v, err := listAPIs(ctx, c, in)
			return answer(ctx, v, err)
		})
}

func bestPlansTool(c *client) fantasy.AgentTool {
	return fantasy.NewAgentTool(BestPlansName, bestPlansAbout,
		func(ctx context.Context, in BestPlansInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			v, err := bestPlans(ctx, c, in)
			return answer(ctx, v, err)
		})
}

func searchPlansTool(c *client) fantasy.AgentTool {
	return fantasy.NewAgentTool(SearchPlansName, searchPlansAbout,
		func(ctx context.Context, in SearchPlansInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			v, err := searchPlans(ctx, c, in)
			return answer(ctx, v, err)
		})
}

func providerTool(c *client) fantasy.AgentTool {
	return fantasy.NewAgentTool(ProviderName, providerAbout,
		func(ctx context.Context, in ProviderInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			v, err := provider(ctx, c, in)
			return answer(ctx, v, err)
		})
}

func listAPIs(ctx context.Context, c *client, in ListAPIsInput) (map[string]any, error) {
	var apis []map[string]any
	if err := c.get(ctx, "/apis", nil, &apis); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(in.Query)
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(query), "-", " "))
	type held struct {
		matched int
		api     map[string]any
	}
	var found []held
	for _, api := range apis {
		name := strings.ToLower(str(api["api"]) + " " + strings.ReplaceAll(str(api["slug"]), "-", " "))
		matched := 0
		for _, w := range words {
			if strings.Contains(name, w) {
				matched++
			}
		}
		if matched > 0 || len(words) == 0 {
			found = append(found, held{matched, brief(api, []string{"api", "slug", "plans", "providers", "best", "bestPage"})})
		}
	}
	// The most words matched first, the sheet's order within each.
	sort.SliceStable(found, func(i, j int) bool { return found[i].matched > found[j].matched })
	limit := 10
	if in.Limit > 0 {
		limit = clamp(int(in.Limit), 1, 200)
	}
	out := []map[string]any{}
	for i := 0; i < len(found) && i < limit; i++ {
		out = append(out, found[i].api)
	}
	v := map[string]any{"total": len(found), "apis": out}
	if len(words) > 0 && (len(found) == 0 || found[0].matched < len(words)) {
		// A provider's name sent as an API's, like Bright Data, whose data matches many APIs: where its plans are.
		v["note"] = "No API's name holds every word of " + strconv.Quote(query) + ". A provider's plans: datacircle_search_plans with its name as query, or datacircle_provider."
	}
	return v, nil
}

// forProvider tells a model that sent a provider's name to an API's tool where a provider's plans are.
const forProvider = "A provider's plans: datacircle_search_plans with its name as query, like bright data, or datacircle_provider."

func bestPlans(ctx context.Context, c *client, in BestPlansInput) (map[string]any, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return nil, advice("Give an API's slug, like email-finder: datacircle_list_apis gives every slug. " + forProvider)
	}
	q := url.Values{}
	if in.Limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(int(in.Limit), 1, 50)))
	}
	var v map[string]any
	if err := c.get(ctx, "/apis/"+slug+"/best", q, &v); err != nil {
		if e, ok := err.(*apiError); ok && e.status == 404 {
			// A provider's name sent as the API's slug: the API's words, and where a provider's plans are.
			return nil, advice(e.message + ". datacircle_list_apis gives every API's slug. " + forProvider)
		}
		return nil, err
	}
	v["plans"] = briefPlans(plans(v["plans"]))
	return v, nil
}

func searchPlans(ctx context.Context, c *client, in SearchPlansInput) (map[string]any, error) {
	api, query, note := strings.TrimSpace(in.API), strings.TrimSpace(in.Query), ""
	sortBy, limit, offset := strings.TrimSpace(in.Sort), int(in.Limit), int(in.Offset)
	v, err := search(ctx, c, api, query, sortBy, limit, offset)
	if e, ok := err.(*apiError); ok && e.status == 400 && strings.Contains(e.message, "api must be") {
		if query != "" {
			// The API's answer names all 150 APIs: where they are instead.
			return nil, advice("No API is named " + strconv.Quote(api) + ". datacircle_list_apis gives every API's name and slug.")
		}
		// A provider's name sent as the API: the plans whose provider or plan name holds it.
		api, query = "", api
		v, err = search(ctx, c, api, query, sortBy, limit, offset)
		note = "No API is named " + strconv.Quote(query) + ": these are the plans whose provider or plan name holds it."
	}
	if total, _ := v["total"].(float64); err == nil && total == 0 && strings.Contains(query, "-") {
		// A slug sent as a name, like bright-data, which no name holds: its words, like bright data. A name with a hyphen,
		// like A-Leads, is found as it is first.
		words := strings.Join(strings.Fields(strings.ReplaceAll(query, "-", " ")), " ")
		if again, err := search(ctx, c, api, words, sortBy, limit, offset); err == nil {
			v = again
			note = strings.TrimSpace(note + " No provider's or plan's name holds " + strconv.Quote(query) + ": these are the plans whose name holds " + strconv.Quote(words) + ".")
		}
	}
	if err != nil {
		return nil, err
	}
	if note != "" {
		v["note"] = note
	}
	v["plans"] = briefPlans(plans(v["plans"]))
	return v, nil
}

func search(ctx context.Context, c *client, api, query, sort string, limit, offset int) (map[string]any, error) {
	q := url.Values{}
	if api != "" {
		q.Set("api", api)
	}
	if query != "" {
		q.Set("query", query)
	}
	if sort == "score" || sort == "provider" || sort == "price" {
		q.Set("sort", sort)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(limit, 1, 50)))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var v map[string]any
	err := c.get(ctx, "/plans", q, &v)
	return v, err
}

func provider(ctx context.Context, c *client, in ProviderInput) (map[string]any, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return nil, advice("Give a provider's slug or name, like bright-data or Bright Data")
	}
	var v map[string]any
	if err := c.get(ctx, "/providers/"+slug, nil, &v); err != nil {
		if e, ok := err.(*apiError); ok && e.status == 404 {
			// The API's words name its own address: the tool that finds a provider's name instead.
			return nil, advice(e.message + ". datacircle_search_plans with words of its name as query gives each plan's provider.")
		}
		return nil, err
	}
	apis := []map[string]any{}
	for _, entry := range plans(v["apis"]) {
		ps := plans(entry["plans"])
		kept := brief(entry, []string{"api", "slug", "rank", "bestPage"})
		kept["plans"] = len(ps)
		if best := bestPlan(ps); best != nil {
			kept["best"] = brief(best, []string{"plan", "score", "perThousand", "page"})
		}
		apis = append(apis, kept)
	}
	// Its best rank first, the APIs where it has none last, in the API's order.
	sort.SliceStable(apis, func(i, j int) bool {
		ri, iok := apis[i]["rank"].(float64)
		rj, jok := apis[j]["rank"].(float64)
		return iok && (!jok || ri < rj)
	})
	ranked := 0
	var bestAt []string
	for _, api := range apis {
		if r, ok := api["rank"].(float64); ok {
			ranked++
			if r == apis[0]["rank"] {
				bestAt = append(bestAt, str(api["api"]))
			}
		}
	}
	limit := 10
	if in.Limit > 0 {
		limit = clamp(int(in.Limit), 1, 200)
	}
	v["total"] = len(apis)
	v["ranked"] = ranked
	// The counts in words, first of the answer's keys: gpt-oss:120b read total, after 63 APIs, as where it is ranked.
	name := str(v["provider"])
	if name == "" {
		name = slug
	}
	about := fmt.Sprintf("%s has plans on %d APIs on Datacircle and a rank on %d of them (1 is the best).", name, len(apis), ranked)
	if ranked > 0 {
		about += fmt.Sprintf(" Its best rank: %v, on %s.", apis[0]["rank"], strings.Join(bestAt, ", "))
	}
	v["about"] = about
	if len(apis) > limit {
		apis = apis[:limit]
	}
	v["apis"] = apis
	return v, nil
}

// bestPlan is a provider's best plan on an API: its lowest rank, a plan with no rank having none.
func bestPlan(ps []map[string]any) map[string]any {
	var best map[string]any
	low := 0.0
	for _, p := range ps {
		if r, ok := p["rank"].(float64); ok && (best == nil || r < low) {
			best, low = p, r
		}
	}
	if best == nil && len(ps) > 0 {
		best = ps[0]
	}
	return best
}

func plans(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
