package datacircle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func TestInfo(t *testing.T) {
	names := []string{ListAPIsName, BestPlansName, SearchPlansName, ProviderName}
	one := []fantasy.AgentTool{NewListAPIsTool(nil), NewBestPlansTool(nil), NewSearchPlansTool(nil), NewProviderTool(nil)}
	for _, tools := range [][]fantasy.AgentTool{NewTools(nil), one} {
		if len(tools) != 4 {
			t.Fatalf("%d tools, want 4", len(tools))
		}
		for i, tl := range tools {
			info := tl.Info()
			if info.Name != names[i] {
				t.Errorf("tool %d is %s, want %s", i, info.Name, names[i])
			}
			params, _ := json.Marshal(info.Parameters)
			required := strings.Join(info.Required, ",")
			switch info.Name {
			case BestPlansName, ProviderName:
				if required != "slug" {
					t.Errorf("%s requires %q, want slug", info.Name, required)
				}
			default:
				if required != "" {
					t.Errorf("%s requires %q, want nothing", info.Name, required)
				}
			}
			if info.Name == SearchPlansName && !strings.Contains(string(params), `"enum":["score","provider","price"]`) {
				t.Errorf("%s's sort has no enum: %s", info.Name, params)
			}
			if !strings.Contains(string(params), `"limit":{"description":"How many`) || !strings.Contains(string(params), `"type":"integer"`) {
				t.Errorf("%s's limit is no described integer: %s", info.Name, params)
			}
			if info.Description == "" {
				t.Errorf("%s has no description", info.Name)
			}
		}
	}
}

func TestCalls(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch {
		case r.URL.Path == "/apis":
			w.Write([]byte(`[{"api":"Employee profile","slug":"linkedin-profile","sheetPage":"s"},{"api":"Employee email","slug":"email-finder","plans":3}]`))
		case r.URL.Path == "/apis/email-finder/best":
			w.Write([]byte(`{"api":"Employee email","plans":[{"provider":"A","rank":1,"terms":"long"}]}`))
		case r.URL.Path == "/plans" && (r.URL.Query().Get("api") == "Bright Data" || r.URL.Query().Get("api") == "bright-data"):
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"api must be one of: Employee email"}`))
		case r.URL.Path == "/plans" && strings.Contains(r.URL.Query().Get("query"), "-"):
			w.Write([]byte(`{"total":0,"plans":[]}`))
		case r.URL.Path == "/plans":
			w.Write([]byte(`{"total":1,"plans":[{"provider":"Bright Data","rank":2}]}`))
		case r.URL.Path == "/providers/bright-data":
			w.Write([]byte(`{"provider":"Bright Data","apis":[{"api":"Employee profile","plans":[{"plan":"C"}]},{"api":"Employee email","rank":4,"plans":[{"plan":"B","rank":9},{"plan":"A","rank":4}]}]}`))
		case r.URL.Path == "/providers/down":
			w.WriteHeader(502)
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"no API has the slug nope"}`))
		}
	}))
	defer server.Close()
	cfg := &Config{BaseURL: server.URL}
	ctx := context.Background()
	for _, call := range []struct {
		tool  fantasy.AgentTool
		input string
		fails bool
		has   string
	}{
		{NewListAPIsTool(cfg), `{"query":"email"}`, false, `"total":1`},
		{NewListAPIsTool(cfg), `{}`, false, `"total":2`},
		{NewBestPlansTool(cfg), `{"slug":"Email Finder","limit":"2"}`, false, `"provider":"A"`},
		{NewSearchPlansTool(cfg), `{"api":"Bright Data"}`, false, `No API is named`},
		{NewSearchPlansTool(cfg), `{"api":"Bright Data","query":"growth"}`, true, `No API is named "Bright Data". datacircle_list_apis gives every API's name and slug.`},
		{NewSearchPlansTool(cfg), `{"api":"bright-data","query":null}`, false, `these are the plans whose name holds \"bright data\".","plans":[{"provider":"Bright Data"`},
		{NewSearchPlansTool(cfg), `{"query":"bright-data","sort":"price","limit":null}`, false, `"plans":[{"provider":"Bright Data"`},
		{NewProviderTool(cfg), `{"slug":"Bright Data"}`, false, `"apis":[{"api":"Employee email","best":{"plan":"A"`},
		{NewProviderTool(cfg), `{"slug":"bright-data","limit":1}`, false, `"ranked":1,"total":2`},
		{NewProviderTool(cfg), `{"slug":"bright-data","limit":1}`, false, `{"about":"Bright Data has plans on 2 APIs on Datacircle and a rank on 1 of them (1 is the best). Its best rank: 4, on Employee email.","apis":[`},
		{NewBestPlansTool(cfg), `{"slug":"nope"}`, true, `no API has the slug nope. datacircle_list_apis gives every API's slug. A provider's plans: datacircle_search_plans`},
		{NewBestPlansTool(cfg), `{"provider":"Bright Data","limit":2}`, true, `Give an API's slug, like email-finder: datacircle_list_apis gives every slug. A provider's plans:`},
		{NewListAPIsTool(cfg), `{"query":"Bright Data"}`, false, `"note":"No API's name holds every word of \"Bright Data\". A provider's plans: datacircle_search_plans`},
		{NewProviderTool(cfg), `{"slug":"down"}`, true, `datacircle: 502 Bad Gateway`},
		{NewBestPlansTool(cfg), `{"slug":"email-finder","limit":[2]}`, true, `invalid parameters`},
		{NewProviderTool(cfg), `{"slug":"nope"}`, true, `no API has the slug nope. datacircle_search_plans with words of its name as query gives each plan's provider.`},
	} {
		name := call.tool.Info().Name
		got, err := call.tool.Run(ctx, fantasy.ToolCall{ID: "c", Name: name, Input: call.input})
		if err != nil || got.IsError != call.fails || !strings.Contains(got.Content, call.has) {
			t.Errorf("%s(%s) = %s (IsError %v), %v; want it to hold %s", name, call.input, got.Content, got.IsError, err, call.has)
		}
		if strings.Contains(got.Content, "terms") || strings.Contains(got.Content, "sheetPage") {
			t.Errorf("%s gave a key it drops: %s", name, got.Content)
		}
	}
	got, _ := NewListAPIsTool(cfg).Run(ctx, fantasy.ToolCall{Input: `{"query":"employee email"}`})
	if strings.Contains(got.Content, "note") {
		t.Errorf("datacircle_list_apis notes a query an API's name holds every word of: %s", got.Content)
	}
	got, _ = NewProviderTool(cfg).Run(ctx, fantasy.ToolCall{Input: `{"slug":"bright-data","limit":1}`})
	if strings.Count(got.Content, `"api":`) != 1 {
		t.Errorf("datacircle_provider with limit 1 gave %s", got.Content)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := NewListAPIsTool(cfg).Run(canceled, fantasy.ToolCall{Input: `{}`}); err == nil {
		t.Errorf("a canceled call gave no error")
	}
	if want := "/apis/email-finder/best?limit=2"; paths[2] != want {
		t.Errorf("called %s, want %s", paths[2], want)
	}
	if want := "/plans?query=bright+data&sort=price"; paths[10] != want {
		t.Errorf("called %s, want %s", paths[10], want)
	}
}
