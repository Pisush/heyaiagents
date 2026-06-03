// Package prompts is the single home for every LLM prompt used by the backend.
//
// Hard rule (see CLAUDE.md): no inline prompt strings may appear anywhere else
// in the codebase. Each prompt is defined here as a typed builder or template
// so it can be reviewed, versioned, and tested independently of the agent task
// functions that consume it. Agent and coverage/networking prompts are added in
// later milestones.
package prompts
