package fusion

import _ "embed"

//go:embed prompts/deliberator.md
var deliberatorPrompt string

//go:embed prompts/judge.md
var judgePrompt string

//go:embed prompts/synthesizer.md
var synthesizerPrompt string

//go:embed prompts/formats/claude.tmpl
var claudeTemplate string

//go:embed prompts/formats/gpt.tmpl
var gptTemplate string

//go:embed prompts/formats/glm.tmpl
var glmTemplate string

//go:embed prompts/formats/qwen.tmpl
var qwenTemplate string

var familyTemplates = map[string]string{
	"claude": claudeTemplate,
	"gpt":    gptTemplate,
	"glm":    glmTemplate,
	"qwen":   qwenTemplate,
}
