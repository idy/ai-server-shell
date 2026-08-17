//go:build compatibility

package openai_compatibility

type liveCase struct {
	Name            string
	Cost            bool
	Mutation        bool
	CleanupRequired bool
}

var liveCases = map[string]liveCase{
	"safe":     {Name: "models.list"},
	"paid":     {Name: "embeddings.create", Cost: true},
	"mutation": {Name: "files.create-delete", Cost: true, Mutation: true, CleanupRequired: true},
}
