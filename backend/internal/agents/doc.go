// Package agents holds the agent task functions for HeyAI Agents.
//
// Each task is a pure-ish server-side function of the form
// (inputs) -> validated struct: runnable, re-runnable, and testable. The two
// MVP capabilities live here in later milestones:
//
//   - Coverage:   (AgentProfile, SessionContent) -> Summary
//   - Networking: (thisProfile, otherProfiles, goals) -> []MatchProposal
//
// All prompts come from internal/prompts; all LLM access goes through
// internal/llm. Outputs are validated at the boundary before they are trusted.
package agents
